package desktop

import (
	"errors"
	"math"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	createWindowEx       = user32.NewProc("CreateWindowExW")
	destroyWindow        = user32.NewProc("DestroyWindow")
	setWindowPos         = user32.NewProc("SetWindowPos")
	showWindow           = user32.NewProc("ShowWindow")
	getFocus             = user32.NewProc("GetFocus")
	setFocus             = user32.NewProc("SetFocus")
	getForegroundWindow  = user32.NewProc("GetForegroundWindow")
	isChild              = user32.NewProc("IsChild")
	isWindow             = user32.NewProc("IsWindow")
	sendMessage          = user32.NewProc("SendMessageW")
	hideCaret            = user32.NewProc("HideCaret")
	validateRect         = user32.NewProc("ValidateRect")
	getKeyState          = user32.NewProc("GetKeyState")
	getDPIForWindow      = user32.NewProc("GetDpiForWindow")
	comctl32             = windows.NewLazySystemDLL("comctl32.dll")
	setWindowSubclass    = comctl32.NewProc("SetWindowSubclass")
	removeWindowSubclass = comctl32.NewProc("RemoveWindowSubclass")
	defSubclassProc      = comctl32.NewProc("DefSubclassProc")
	imm32                = windows.NewLazySystemDLL("imm32.dll")
	immGetContext        = imm32.NewProc("ImmGetContext")
	immReleaseContext    = imm32.NewProc("ImmReleaseContext")
	immNotifyIME         = imm32.NewProc("ImmNotifyIME")
	immGetComposition    = imm32.NewProc("ImmGetCompositionStringW")
	immSetCandidate      = imm32.NewProc("ImmSetCandidateWindow")
	immSetComposition    = imm32.NewProc("ImmSetCompositionWindow")
	inputControls        sync.Map
	inputSubclass        = syscall.NewCallback(inputWindowProc)
)

type nativeInput struct {
	inputState
	hwnd, parent, previous uintptr
	height                 int32
}

func newNativeInput(parent uintptr, publish func(InputEvent)) (*nativeInput, error) {
	input := &nativeInput{parent: parent, inputState: inputState{publish: publish, modifiers: keyboardModifiers}}
	control, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("EDIT"))), 0, 0x40000000|0x0004, 0, 0, 1, 24, parent, 0, 0, 0)
	if control == 0 {
		return nil, fmtNative("无法创建原生输入控件", err)
	}
	input.hwnd = control
	inputControls.Store(control, input)
	result, _, err := setWindowSubclass.Call(control, inputSubclass, 1, 0)
	if result == 0 {
		inputControls.Delete(control)
		destroyWindow.Call(control)
		return nil, fmtNative("无法连接原生输入控件", err)
	}
	return input, nil
}

func keyboardModifiers() (bool, bool, bool, bool, bool) {
	pressed := func(key uintptr) bool { value, _, _ := getKeyState.Call(key); return value&0x8000 != 0 }
	caps, _, _ := getKeyState.Call(20)
	return pressed(17), pressed(18), pressed(16), pressed(91) || pressed(92), caps&1 != 0
}

func (input *nativeInput) close() {
	if input.hwnd == 0 {
		return
	}
	input.cancel()
	removeWindowSubclass.Call(input.hwnd, inputSubclass, 1)
	inputControls.Delete(input.hwnd)
	destroyWindow.Call(input.hwnd)
	input.hwnd = 0
}

func (input *nativeInput) cancel() {
	if input.composing {
		input.emit("compositionend", "", 0, 0)
	}
	input.composing = false
	input.resetting = true
	context, _, _ := immGetContext.Call(input.hwnd)
	if context != 0 {
		immNotifyIME.Call(context, 0x15, 4, 0)
		immReleaseContext.Call(input.hwnd, context)
	}
	sendMessage.Call(input.hwnd, 0x000c, 0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(""))))
	input.high = 0
	input.resetting = false
}

func (input *nativeInput) deactivate() {
	if input.hwnd == 0 {
		return
	}
	input.cancel()
	input.emit("deactivated", "", 0, 0)
	input.owner = ""
	showWindow.Call(input.hwnd, 0)
}

func (input *nativeInput) request(request RemoteInputRequest, zoom float64) (RemoteInputResult, error) {
	var result RemoteInputResult
	if input.hwnd == 0 {
		return result, errors.New("原生输入控件尚未就绪")
	}
	if !validInput(request) {
		return result, nil
	}
	if request.Operation != "activate" && request.Owner != input.owner {
		return result, nil
	}
	if request.Operation == "activate" || request.Operation == "caret" {
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(input.parent)
		minimised, _, _ := user32.NewProc("IsIconic").Call(input.parent)
		if visible == 0 || minimised != 0 {
			return result, nil
		}
	}
	switch request.Operation {
	case "activate", "caret":
		if request.Operation == "activate" && input.owner != request.Owner {
			input.cancel()
			input.owner = request.Owner
		}
		dpi, _, _ := getDPIForWindow.Call(input.parent)
		if dpi == 0 {
			dpi = 96
		}
		scale := float64(dpi) / 96 * zoom
		x, y, height := int32(request.X*scale), int32(request.Y*scale), int32(max(1, math.Round(request.Height*scale)))
		input.height = height
		setWindowPos.Call(input.hwnd, 0, uintptr(x), uintptr(y), 1, uintptr(height), 0x0010)
		if request.Operation == "activate" {
			focused, _, _ := getFocus.Call()
			if focused != input.hwnd {
				input.previous = focused
			}
			showWindow.Call(input.hwnd, 5)
			setFocus.Call(input.hwnd)
		}
		input.positionCandidates()
	case "cancel":
		input.cancel()
	case "deactivate":
		focused, _, _ := getFocus.Call()
		foreground, _, _ := getForegroundWindow.Call()
		hadFocus := focused == input.hwnd && foreground == input.parent
		input.cancel()
		input.owner = ""
		showWindow.Call(input.hwnd, 0)
		if request.ReturnFocus && hadFocus && input.previous != 0 {
			valid, _, _ := isWindow.Call(input.previous)
			child, _, _ := isChild.Call(input.parent, input.previous)
			if valid != 0 && child != 0 {
				setFocus.Call(input.previous)
				focused, _, _ = getFocus.Call()
				result.ReturnedFocus = focused == input.previous
			}
		}
	}
	result.Accepted = true
	return result, nil
}

func inputWindowProc(hwnd uintptr, message uint32, wParam, lParam, subclass, reference uintptr) uintptr {
	value, ok := inputControls.Load(hwnd)
	if !ok {
		result, _, _ := defSubclassProc.Call(hwnd, uintptr(message), wParam, lParam)
		return result
	}
	input := value.(*nativeInput)
	switch message {
	case 0x000f:
		validateRect.Call(hwnd, 0)
		return 0
	case 0x0087:
		return 0x0004 | 0x0002 | 0x0001 | 0x0080 // DLGC_WANTALLKEYS|TAB|ARROWS|CHARS
	case 0x0008:
		input.deactivate()
	case 0x0100, 0x0104:
		input.readKey(uint32(wParam), lParam)
		ctrl, alt, _, _, _ := keyboardModifiers()
		if !input.composing && (ctrl || alt || commandKey(uint32(wParam))) {
			input.key(uint32(wParam), true, "")
			return 0
		}
	case 0x0101, 0x0105:
		input.readKey(uint32(wParam), lParam)
		input.key(uint32(wParam), false, "")
		return 0
	case 0x0102, 0x0106:
		if !input.composing {
			ctrl, alt, _, _, _ := keyboardModifiers()
			if !ctrl && !alt {
				input.character(uint16(wParam))
			}
			return 0
		}
	case 0x010d:
		if !input.resetting {
			input.composing = true
			input.emit("compositionstart", "", 0, 0)
		}
	case 0x010e:
		if input.composing && !input.resetting {
			input.composing = false
			input.emit("compositionend", "", 0, 0)
		}
	case 0x010f:
		if input.readComposition(lParam) {
			return 0
		}
	}
	result, _, _ := defSubclassProc.Call(hwnd, uintptr(message), wParam, lParam)
	if input.owner != "" && (message == 0x010d || message == 0x010f || message == 7 || message == 5 || message == 3) {
		input.positionCandidates()
	}
	hideCaret.Call(hwnd)
	return result
}

// A committed RESULTSTR is emitted once and must not reach EDIT's default
// handler, which would insert the same text again through WM_CHAR.
func (input *nativeInput) readComposition(flags uintptr) bool {
	if input.resetting || input.owner == "" {
		return false
	}
	// Some TSF providers send RESULTSTR after focus has moved but before
	// WM_KILLFOCUS reaches EDIT. That late result must not reach the remote
	// field or EDIT's default handler after the composition was abandoned.
	focused, _, _ := getFocus.Call()
	if focused != input.hwnd {
		input.deactivate()
		return true
	}
	context, _, _ := immGetContext.Call(input.hwnd)
	if context == 0 {
		return false
	}
	committed := flags&0x800 != 0
	if committed {
		input.emit("compositionend", compositionText(context, 0x800), 0, 0)
		input.composing = false
	}
	if flags&8 != 0 {
		if !input.composing {
			input.composing = true
			input.emit("compositionstart", "", 0, 0)
		}
		text := compositionText(context, 8)
		cursor, _, _ := immGetComposition.Call(context, 0x80, 0, 0)
		position := max(0, min(int(int32(cursor)), len(utf16.Encode([]rune(text)))))
		input.emit("compositionupdate", text, position, position)
	}
	immReleaseContext.Call(input.hwnd, context)
	input.positionCandidates()
	if !committed || input.composing {
		return committed
	}
	input.resetting = true
	sendMessage.Call(input.hwnd, 0x000c, 0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(""))))
	input.resetting = false
	return true
}

func compositionText(context uintptr, index uintptr) string {
	length, _, _ := immGetComposition.Call(context, index, 0, 0)
	if int32(length) <= 0 || length > 16384 || length%2 != 0 {
		return ""
	}
	data := make([]uint16, length/2)
	count, _, _ := immGetComposition.Call(context, index, uintptr(unsafe.Pointer(&data[0])), length)
	if int32(count) <= 0 || count > length {
		return ""
	}
	return string(utf16.Decode(data[:count/2]))
}

type winPoint struct{ X, Y int32 }
type winRect struct{ Left, Top, Right, Bottom int32 }

func (input *nativeInput) positionCandidates() {
	context, _, _ := immGetContext.Call(input.hwnd)
	if context == 0 {
		return
	}
	defer immReleaseContext.Call(input.hwnd, context)
	candidate := struct {
		Index, Style uint32
		Point        winPoint
		Area         winRect
	}{Style: 0x80, Point: winPoint{Y: input.height}, Area: winRect{Right: 1, Bottom: input.height}}
	composition := struct {
		Style uint32
		Point winPoint
		Area  winRect
	}{Style: 2}
	immSetComposition.Call(context, uintptr(unsafe.Pointer(&composition)))
	immSetCandidate.Call(context, uintptr(unsafe.Pointer(&candidate)))
}
