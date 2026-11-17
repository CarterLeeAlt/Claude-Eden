package desktop

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

var (
	windowsGDI       = windows.NewLazySystemDLL("gdi32.dll")
	dialogClassOnce  sync.Once
	dialogClassError error
	dialogWindows    sync.Map
	dialogCallback   = syscall.NewCallback(dialogWindowProc)
)

type nativeDialog struct {
	window, parent uintptr
	accepted, done bool
	confirmation   bool
	timer          func(uintptr)
}
type nativeDialogOptions struct {
	title, message string
	confirmation   bool
	fonts          *nativeFonts
	timer          func(uintptr)
}

// ShowStartupError displays failures that happen before Wails has a window.
// Automated smoke/self-test modes report errors through their result contract.
func ShowStartupError(message string, options Options) error {
	fonts, err := newNativeFonts(options.FontRegular, options.FontBold)
	if err != nil {
		return err
	}
	defer fonts.close()
	return showStartupError(message, fonts, nil)
}

func showStartupError(message string, fonts *nativeFonts, timer func(uintptr)) error {
	_, err := showNativeDialog(0, nativeDialogOptions{title: "Claude Eden 启动失败", message: message, fonts: fonts, timer: timer})
	return err
}

func showNativeDialog(parent uintptr, options nativeDialogOptions) (bool, error) {
	if options.fonts == nil {
		return false, errors.New("原生对话框缺少内置界面字体")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dialogClassOnce.Do(func() {
		class := w32.WNDCLASSEX{WndProc: dialogCallback, ClassName: windows.StringToUTF16Ptr("EdenNativeDialog"), Background: w32.HBRUSH(6)}
		class.Size = uint32(unsafe.Sizeof(class))
		if w32.RegisterClassEx(&class) == 0 {
			dialogClassError = errors.New("无法注册原生对话框")
		}
	})
	if dialogClassError != nil {
		return false, dialogClassError
	}
	dpi, _, _ := getDPIForWindow.Call(parent)
	if dpi == 0 {
		dpi = 96
	}
	unit := func(value int32) int32 { return value * int32(dpi) / 96 }
	var monitor struct {
		Size          uint32
		Monitor, Work winRect
		Flags         uint32
	}
	monitor.Size = uint32(unsafe.Sizeof(monitor))
	handle, _, _ := monitorFromWindow.Call(parent, 1)
	if result, _, nativeError := getMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&monitor))); result == 0 {
		return false, fmtNative("无法读取对话框工作区", nativeError)
	}
	anchor := monitor.Work
	if parent != 0 {
		if result, _, nativeError := user32.NewProc("GetWindowRect").Call(parent, uintptr(unsafe.Pointer(&anchor))); result == 0 {
			return false, fmtNative("无法读取对话框所属窗口位置", nativeError)
		}
	}
	fonts := options.fonts
	font := fonts.font(int(dpi), false)
	width := min(unit(620), monitor.Work.Right-monitor.Work.Left-unit(32))
	textWidth := width - unit(48)
	text := windows.StringToUTF16(options.message)
	dc := w32.GetDC(w32.HWND(parent))
	previous := w32.SelectObject(dc, w32.HGDIOBJ(font))
	textRect := w32.RECT{Right: textWidth}
	w32.DrawText(dc, text, len(text)-1, &textRect, 0x0400|0x0010|0x0800) // CALCRECT|WORDBREAK|NOPREFIX.
	w32.SelectObject(dc, previous)
	w32.ReleaseDC(w32.HWND(parent), dc)
	textHeight := max(unit(28), min(unit(360), monitor.Work.Bottom-monitor.Work.Top-unit(172), textRect.Bottom-textRect.Top))
	height := textHeight + unit(140)
	x := max(monitor.Work.Left, min(monitor.Work.Right-width, anchor.Left+(anchor.Right-anchor.Left-width)/2))
	y := max(monitor.Work.Top, min(monitor.Work.Bottom-height, anchor.Top+(anchor.Bottom-anchor.Top-height)/2))
	dialog := &nativeDialog{parent: parent, confirmation: options.confirmation, timer: options.timer}
	hwnd, _, err := createWindowEx.Call(0x00000001, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("EdenNativeDialog"))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(options.title))), 0x80c80000, uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, 0, 0, 0)
	if hwnd == 0 {
		return false, fmtNative("无法创建原生对话框", err)
	}
	dialog.window = hwnd
	dialogWindows.Store(hwnd, dialog)
	defer func() {
		dialogWindows.Delete(hwnd)
		destroyWindow.Call(hwnd)
		user32.NewProc("EnableWindow").Call(parent, 1)
		setFocus.Call(parent)
	}()
	user32.NewProc("EnableWindow").Call(parent, 0)
	textStyle := uintptr(0x50000804 | 0x40)
	if textRect.Bottom-textRect.Top > textHeight {
		textStyle |= 0x00200000
	}
	textWindow, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("EDIT"))), uintptr(unsafe.Pointer(&text[0])), textStyle, uintptr(unit(24)), uintptr(unit(20)), uintptr(textWidth), uintptr(textHeight), hwnd, 0, 0, 0)
	if textWindow == 0 {
		return false, fmtNative("无法创建对话框文本", err)
	}
	sendMessage.Call(textWindow, 0x30, uintptr(font), 1)
	buttonY := textHeight + unit(40)
	buttonWidth := unit(92)
	acceptX := width - unit(24) - buttonWidth
	if options.confirmation {
		acceptX -= buttonWidth + unit(12)
	}
	accept, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("BUTTON"))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("确定"))), 0x50010001, uintptr(acceptX), uintptr(buttonY), uintptr(buttonWidth), uintptr(unit(34)), hwnd, 1, 0, 0)
	if accept == 0 {
		return false, fmtNative("无法创建对话框按钮", err)
	}
	sendMessage.Call(accept, 0x30, uintptr(font), 1)
	if options.confirmation {
		cancel, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("BUTTON"))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("取消"))), 0x50010000, uintptr(acceptX+buttonWidth+unit(12)), uintptr(buttonY), uintptr(buttonWidth), uintptr(unit(34)), hwnd, 2, 0, 0)
		if cancel == 0 {
			return false, fmtNative("无法创建取消按钮", err)
		}
		sendMessage.Call(cancel, 0x30, uintptr(font), 1)
	}
	showWindow.Call(hwnd, 5)
	user32.NewProc("SetForegroundWindow").Call(hwnd)
	setFocus.Call(accept)
	if dialog.timer != nil {
		user32.NewProc("SetTimer").Call(hwnd, 1, 200, 0)
	}
	for !dialog.done {
		var message w32.MSG
		code := w32.GetMessage(&message, 0, 0, 0)
		if code <= 0 {
			if code == 0 {
				user32.NewProc("PostQuitMessage").Call(uintptr(message.WParam))
			}
			return false, errors.New("对话框消息循环已退出")
		}
		if message.Message == 0x0100 && (message.WParam == 13 || message.WParam == 27) {
			dialog.accepted = message.WParam == 13 || !options.confirmation
			dialog.done = true
			continue
		}
		handled, _, _ := user32.NewProc("IsDialogMessageW").Call(hwnd, uintptr(unsafe.Pointer(&message)))
		if handled == 0 {
			w32.TranslateMessage(&message)
			w32.DispatchMessage(&message)
		}
	}
	return dialog.accepted, nil
}

func dialogWindowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if value, ok := dialogWindows.Load(hwnd); ok {
		dialog := value.(*nativeDialog)
		switch message {
		case 0x0010:
			dialog.accepted = !dialog.confirmation
			dialog.done = true
			return 0
		case 0x0111:
			if wParam&0xffff == 1 || wParam&0xffff == 2 {
				dialog.accepted = wParam&0xffff == 1
				dialog.done = true
				return 0
			}
		case 0x0113:
			if dialog.timer != nil {
				user32.NewProc("KillTimer").Call(hwnd, 1)
				dialog.timer(hwnd)
				return 0
			}
		case 0x0138:
			windowsGDI.NewProc("SetBkColor").Call(wParam, 0xffffff)
			windowsGDI.NewProc("SetTextColor").Call(wParam, 0x1f1c1a)
			return uintptr(w32.GetSysColorBrush(5))
		}
	}
	result, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(message), wParam, lParam)
	return result
}
