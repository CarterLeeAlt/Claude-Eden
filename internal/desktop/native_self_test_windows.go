package desktop

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

type NativeSelfTestReport struct {
	Success        bool         `json:"success"`
	Machine        string       `json:"machine"`
	VirtualMachine string       `json:"virtualMachine"`
	DeviceDPI      uint32       `json:"deviceDpi"`
	Checks         []string     `json:"checks"`
	Errors         []string     `json:"errors"`
	Limitations    []string     `json:"limitations"`
	Notes          []string     `json:"notes"`
	Screenshots    []string     `json:"screenshots"`
	InputEvents    []InputEvent `json:"inputEvents"`
}

// RunNativeSelfTest is an explicitly opt-in VM-only integration entry. Normal
// go test and ordinary desktop startup never invoke it. It modifies the VM's
// clipboard and focuses only its own window; it restores the clipboard when
// the original formats are representable, and refuses otherwise.
func RunNativeSelfTest(output string, options Options) (report NativeSelfTestReport, err error) {
	return runNativeSelfTest(output, true, options)
}

func RunNativeInputSelfTest(output string, options Options) (NativeSelfTestReport, error) {
	return runNativeSelfTest(output, false, options)
}

func runNativeSelfTest(output string, webview bool, options Options) (report NativeSelfTestReport, err error) {
	machine, e := nativeTestMachine()
	if e != nil {
		return report, e
	}
	output, e = filepath.Abs(output)
	if e != nil {
		return report, e
	}
	if output == filepath.VolumeName(output)+string(filepath.Separator) {
		return report, errors.New("自检输出必须是独立目录")
	}
	if e = os.MkdirAll(output, 0700); e != nil {
		return report, e
	}
	entries, e := os.ReadDir(output)
	if e != nil {
		return report, e
	}
	if len(entries) != 0 {
		return report, errors.New("原生自检输出目录必须为空，避免复用上次 PID、结果或截图")
	}
	report.Machine, _ = os.Hostname()
	report.VirtualMachine = machine
	report.Errors = []string{}
	report.Checks = []string{}
	report.Screenshots = []string{}
	report.Limitations = []string{"实际跨显示器 DPI 切换需要多 DPI 显示设备；本入口验证当前设备 DPI、不同 CSS 缩放下的真实 EDIT 控件与候选框坐标。"}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("原生自检异常：%v", recovered)
		}
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
		report.Success = len(report.Errors) == 0
		data, e := json.MarshalIndent(report, "", "  ")
		if e == nil {
			e = os.WriteFile(filepath.Join(output, "Native_Result.json"), data, 0600)
		}
		if e != nil {
			report.Success = false
			report.Errors = append(report.Errors, "保存原生自检结果："+e.Error())
			err = errors.Join(err, e)
		}
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	fonts, e := newNativeFonts(options.FontRegular, options.FontBold)
	if e != nil {
		return report, e
	}
	defer fonts.close()
	report.Checks = append(report.Checks, "同一内置 Eden UI 常规/粗体私有注册、GDI 字体名与必需字形核对")
	if len(options.Icon) == 0 {
		return report, errors.New("原生自检缺少内置桌面图标")
	}
	largeIcon, e := w32.CreateLargeHIconFromImage(options.Icon)
	if e != nil {
		return report, e
	}
	defer w32.DestroyIcon(largeIcon)
	smallIcon, e := w32.CreateSmallHIconFromImage(options.Icon)
	if e != nil {
		return report, e
	}
	defer w32.DestroyIcon(smallIcon)
	check := func(condition bool, message string) {
		if !condition {
			panic(message)
		}
		report.Checks = append(report.Checks, message)
	}
	for _, index := range []int{1, 5, 15} {
		direct, _, _ := user32.NewProc("GetSysColorBrush").Call(uintptr(index))
		check(direct != 0 && uintptr(w32.GetSysColorBrush(index)) == direct, fmt.Sprintf("GetSysColorBrush 实际系统画刷索引 %d 与原生 API 一致", index))
	}
	className := windows.StringToUTF16Ptr("EdenNativeSelfTest-" + uuid.NewString())
	var testTheme *nativeMenuTheme
	var menuTimer func()
	menuDrawn := false
	callback := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		if testTheme != nil {
			if result, handled := testTheme.hook(hwnd, msg, wParam, lParam); handled {
				if msg == 0x002b {
					menuDrawn = true
				}
				return result
			}
		}
		if msg == 0x0113 && wParam == 0x7201 && menuTimer != nil {
			user32.NewProc("KillTimer").Call(hwnd, wParam)
			menuTimer()
			return 0
		}
		if msg == 0x0010 {
			return 0
		}
		result, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wParam, lParam)
		return result
	})
	class := w32.WNDCLASSEX{WndProc: callback, ClassName: className, Background: w32.HBRUSH(6), Icon: largeIcon, IconSm: smallIcon}
	class.Size = uint32(unsafe.Sizeof(class))
	check(w32.RegisterClassEx(&class) != 0, "Win32 自检窗口注册")
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(className)), 0)
	hwnd, _, nativeErr := createWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Claude Eden 原生交互 VM 自检"))), 0x00cf0000, 100, 100, 900, 560, 0, 0, 0, 0)
	if hwnd == 0 {
		return report, fmtNative("创建自检窗口失败", nativeErr)
	}
	defer destroyWindow.Call(hwnd)
	label, _, nativeErr := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("STATIC"))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Claude Eden 原生交互自检\r\n本窗口仅在测试虚拟机运行。\r\n当前验证：键盘、剪贴板、EDIT 输入法组合与候选定位。"))), 0x50000000, 28, 28, 800, 100, hwnd, 0, 0, 0)
	if label == 0 {
		return report, fmtNative("无法创建原生自检说明控件", nativeErr)
	}
	showWindow.Call(hwnd, 5)
	user32.NewProc("SetForegroundWindow").Call(hwnd)
	pumpNativeMessages(100 * time.Millisecond)
	dpi, _, _ := getDPIForWindow.Call(hwnd)
	report.DeviceDPI = uint32(dpi)
	sendMessage.Call(label, 0x30, uintptr(fonts.font(int(dpi), false)), 1)
	status, e := newNativeStatus(hwnd, fonts)
	if e != nil {
		return report, e
	}
	defer status.close()
	showWindow.Call(label, 0)
	for _, state := range []struct {
		name, message  string
		retry, upgrade bool
	}{
		{"Starting", "正在启动 Claude Eden 工作台…", false, false},
		{"Failed", "无法连接工作台。后台任务保留，可重试连接。", true, false},
		{"Update_Pending", "更新待完成，请打开现有环境保存文件，或保存后停止环境并完成更新。", false, true},
	} {
		status.show(state.message, state.retry, state.upgrade)
		pumpNativeMessages(30 * time.Millisecond)
		if e = verifySelfTestControlFonts(hwnd, fonts); e != nil {
			return report, e
		}
		name := "Native_Startup_" + state.name + ".png"
		if e = captureVisibleNativeWindow(hwnd, filepath.Join(output, name)); e != nil {
			return report, e
		}
		report.Screenshots = append(report.Screenshots, name)
		if e = verifySelfTestStatusButtons(status, filepath.Join(output, name)); e != nil {
			return report, e
		}
	}
	status.hide()
	status.close()
	showWindow.Call(label, 5)
	report.Checks = append(report.Checks, "启动中、失败重试、更新待完成的全部原生状态控件使用同一内置字体并截图")
	var startupDialogError error
	var startupBodyError error
	e = showStartupError("原生启动失败对话框字体验收：Claude Eden 中文繁體日本語 Français Deutsch。", fonts, func(dialog uintptr) {
		startupDialogError = errors.Join(verifySelfTestControlFonts(dialog, fonts), captureVisibleNativeWindow(dialog, filepath.Join(output, "Native_Startup_Error_Visible.png")), captureNativeWindow(dialog, filepath.Join(output, "Native_Startup_Error_Dialog.png")))
		if startupDialogError == nil {
			startupBodyError = verifySelfTestDialogBody(dialog, filepath.Join(output, "Native_Startup_Error_Visible.png"))
		}
		textWindow, _, _ := user32.NewProc("GetDlgItem").Call(dialog, 0)
		text := make([]uint16, 4096)
		count, _, _ := user32.NewProc("GetWindowTextW").Call(textWindow, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		if count == 0 || !strings.Contains(windows.UTF16ToString(text), "原生启动失败对话框字体验收") {
			startupDialogError = errors.Join(startupDialogError, errors.New("启动失败对话框正文控件内容缺失"))
		}
		user32.NewProc("PostMessageW").Call(dialog, 0x0111, 1, 0)
	})
	if e = errors.Join(e, startupDialogError); e != nil {
		return report, e
	}
	report.Checks = append(report.Checks, "实际启动失败对话框的正文和按钮使用同一内置字体")
	report.Screenshots = append(report.Screenshots, "Native_Startup_Error_Dialog.png")
	report.Screenshots = append(report.Screenshots, "Native_Startup_Error_Visible.png")
	if startupBodyError != nil {
		report.Errors = append(report.Errors, startupBodyError.Error())
	}
	user32.NewProc("SetForegroundWindow").Call(hwnd)
	input, e := newNativeInput(hwnd, func(value InputEvent) { report.InputEvents = append(report.InputEvents, value) })
	if e != nil {
		return report, e
	}
	defer input.close()
	owner := uuid.NewString()
	accepted, e := input.request(RemoteInputRequest{Owner: owner, Operation: "activate", X: 80, Y: 180, Height: 24}, 1)
	check(e == nil && accepted.Accepted, "真实 EDIT 子控件激活")

	// SetKeyboardState is thread-local; no SendInput and no LED mutation.
	original := make([]byte, 256)
	check(w32.GetKeyboardState(original), "保存测试线程键盘状态")
	setKeyboard := user32.NewProc("SetKeyboardState")
	defer setKeyboard.Call(uintptr(unsafe.Pointer(&original[0])))
	for _, caps := range []bool{false, true} {
		for _, shift := range []bool{false, true} {
			state := make([]byte, 256)
			if caps {
				state[20] = 1
			}
			if shift {
				state[16] = 0x80
				state[0xa0] = 0x80
			}
			result, _, _ := setKeyboard.Call(uintptr(unsafe.Pointer(&state[0])))
			check(result != 0, "设置测试线程 CapsLock/Shift 组合")
			start := len(report.InputEvents)
			character := 'a'
			if caps != shift {
				character = 'A'
			}
			sendMessage.Call(input.hwnd, 0x100, 0x41, 0x1e<<16)
			sendMessage.Call(input.hwnd, 0x102, uintptr(character), 0)
			sendMessage.Call(input.hwnd, 0x101, 0x41, 0x1e<<16)
			values := report.InputEvents[start:]
			check(len(values) == 2 && values[0].Kind == "keydown" && values[0].Key == string(character) && values[0].Code == "KeyA" && values[0].TextKey && values[1].Kind == "keyup" && values[0].ModifierCapsLock == caps && values[0].ShiftKey == shift, "真实控件 printable keydown/WM_CHAR/keyup 与修饰键")
		}
	}
	setKeyboard.Call(uintptr(unsafe.Pointer(&original[0])))
	for _, test := range []struct {
		key  uint32
		data uintptr
		code string
	}{{13, 1<<24 | 0x1c<<16, "NumpadEnter"}, {16, 0x36 << 16, "ShiftRight"}, {17, 1<<24 | 0x1d<<16, "ControlRight"}, {18, 1<<24 | 0x38<<16, "AltRight"}, {37, 0x4b << 16, "Numpad4"}} {
		start := len(report.InputEvents)
		sendMessage.Call(input.hwnd, 0x100, uintptr(test.key), test.data)
		sendMessage.Call(input.hwnd, 0x101, uintptr(test.key), test.data)
		values := report.InputEvents[start:]
		check(len(values) == 2 && values[0].Code == test.code && values[1].Code == test.code, "真实控件扫描码与左右/数字键盘按键 "+test.code)
	}
	start := len(report.InputEvents)
	sendMessage.Call(input.hwnd, 0x102, 0xd83d, 0)
	sendMessage.Call(input.hwnd, 0x102, 0xde00, 0)
	check(len(report.InputEvents) == start+1 && report.InputEvents[start].Text == "😀", "真实控件 UTF-16 代理对")
	showWindow.Call(hwnd, 0)
	pumpNativeMessages(30 * time.Millisecond)
	if hidden, hiddenError := input.request(RemoteInputRequest{Owner: owner, Operation: "activate", X: 80, Y: 180, Height: 24}, 1); hiddenError != nil || hidden.Accepted {
		return report, errors.New("隐藏窗口接受了迟到的原生输入激活")
	}
	check(input.owner == "" && !input.composing, "隐藏后的原生 EDIT 失焦释放 owner 和组合状态")
	showWindow.Call(hwnd, 5)
	user32.NewProc("SetForegroundWindow").Call(hwnd)
	previousOwner := owner
	owner = uuid.NewString()
	if resumed, resumeError := input.request(RemoteInputRequest{Owner: owner, Operation: "activate", X: 80, Y: 180, Height: 24}, 1); resumeError != nil || !resumed.Accepted {
		return report, errors.New("恢复窗口后新原生输入 owner 未能激活")
	}
	stale, staleError := input.request(RemoteInputRequest{Owner: previousOwner, Operation: "deactivate"}, 1)
	check(staleError == nil && !stale.Accepted && input.owner == owner, "恢复后旧 owner 的迟到释放不影响新 owner")
	focusTarget, _, focusError := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("BUTTON"))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("输入焦点归还自检"))), 0x50010000, 80, 240, 180, 34, hwnd, 0, 0, 0)
	if focusTarget == 0 {
		return report, fmtNative("无法创建自检焦点目标", focusError)
	}
	defer destroyWindow.Call(focusTarget)
	sendMessage.Call(focusTarget, 0x30, uintptr(fonts.font(int(dpi), false)), 1)
	for _, returnFocus := range []bool{false, true} {
		setFocus.Call(focusTarget)
		owner = uuid.NewString()
		activation, activationError := input.request(RemoteInputRequest{Owner: owner, Operation: "activate", X: 80, Y: 180, Height: 24}, 1)
		if activationError != nil || !activation.Accepted {
			return report, errors.New("焦点归还自检无法激活原生输入")
		}
		released, releaseError := input.request(RemoteInputRequest{Owner: owner, Operation: "deactivate", ReturnFocus: returnFocus}, 1)
		focused, _, _ := getFocus.Call()
		check(releaseError == nil && released.Accepted && released.ReturnedFocus == returnFocus && (!returnFocus || focused == focusTarget), fmt.Sprintf("原生输入仅按明确 returnFocus=%t 归还焦点并回读", returnFocus))
	}
	restoreProfile, e := activatePinyinProfile()
	if e != nil {
		return report, e
	}
	restored := false
	restoreTestProfile := func() {
		if !restored {
			restored = true
			if restoreError := restoreProfile(); restoreError != nil {
				report.Errors = append(report.Errors, "恢复原线程输入布局："+restoreError.Error())
			} else {
				report.Checks = append(report.Checks, "恢复 VM 原线程输入语言、键盘布局与 TSF profile 并回读核对")
			}
		}
	}
	defer restoreTestProfile()
	if activation, activationError := input.request(RemoteInputRequest{Owner: owner, Operation: "activate", X: 80, Y: 180, Height: 24}, 1); activationError != nil || !activation.Accepted {
		return report, errors.New("切换微软拼音后原生输入控件未能激活")
	}
	for _, scale := range []float64{1, 1.25, 1.5, 2, 2.5} {
		_, e = input.request(RemoteInputRequest{Owner: owner, Operation: "caret", X: 80, Y: 180, Height: 24}, scale)
		check(e == nil, "CSS/DPI 光标定位")
		rect := w32.GetWindowRect(w32.HWND(input.hwnd))
		expected := int32(24 * float64(dpi) / 96 * scale)
		check(rect != nil && rect.Bottom-rect.Top == expected, fmt.Sprintf("EDIT 实际高度 scale=%.2f", scale))
		context, _, _ := immGetContext.Call(input.hwnd)
		check(context != 0, "EDIT 原生输入法上下文")
		var candidate struct {
			Index, Style uint32
			Point        winPoint
			Area         winRect
		}
		result, _, _ := imm32.NewProc("ImmGetCandidateWindow").Call(context, 0, uintptr(unsafe.Pointer(&candidate)))
		immReleaseContext.Call(input.hwnd, context)
		check(result != 0 && candidate.Style == 0x80 && candidate.Point.Y == expected && candidate.Area.Bottom == expected, "IMM 候选框 CFS_EXCLUDE 坐标回读")
		pumpNativeMessages(30 * time.Millisecond)
		name := fmt.Sprintf("Native_Input_Scale_%03d.png", int(scale*100))
		if e = captureNativeWindow(hwnd, filepath.Join(output, name)); e != nil {
			return report, e
		}
		report.Screenshots = append(report.Screenshots, name)
	}
	start = len(report.InputEvents)
	sendMessage.Call(input.hwnd, 0x10d, 0, 0)
	sendMessage.Call(input.hwnd, 0x10e, 0, 0)
	check(len(report.InputEvents) == start+2 && report.InputEvents[start].Kind == "compositionstart" && report.InputEvents[start+1].Kind == "compositionend", "真实控件组合开始/取消消息")
	if e = exerciseIMMComposition(input, &report); e != nil {
		report.Notes = append(report.Notes, e.Error())
	}
	if e = exerciseRealPinyin(input, output, &report); e != nil {
		report.Errors = append(report.Errors, e.Error())
	}
	input.deactivate()
	restoreErrors := len(report.Errors)
	restoreTestProfile()
	if len(report.Errors) != restoreErrors {
		return report, errors.New("输入法状态恢复失败，停止后续原生交互验收")
	}
	testTheme = &nativeMenuTheme{fonts: fonts, background: application.NewRGB(247, 247, 247), foreground: application.NewRGB(26, 28, 31), dpi: int(dpi)}
	menu := w32.NewPopupMenu()
	if menu == 0 {
		return report, errors.New("无法创建 VM 托盘菜单自检")
	}
	defer w32.DestroyMenu(menu)
	for index, caption := range []string{"打开 Claude Eden", "检查应用更新…", "关于 Claude Eden", "", "退出 Claude Eden…"} {
		flags := uint32(0)
		if caption == "" {
			flags = 0x800
		}
		if !w32.AppendMenu(menu, flags, uintptr(index+1), windows.StringToUTF16Ptr(caption)) {
			return report, errors.New("无法填充 VM 托盘菜单自检")
		}
	}
	var menuError error
	menuShown := false
	menuTimer = func() {
		popup, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("#32768"))), 0)
		var pid uint32
		if popup != 0 {
			user32.NewProc("GetWindowThreadProcessId").Call(popup, uintptr(unsafe.Pointer(&pid)))
		}
		if popup == 0 || pid != uint32(os.Getpid()) {
			menuError = errors.New("没有找到自有可见托盘菜单")
		} else {
			menuShown = true
			user32.NewProc("HiliteMenuItem").Call(hwnd, uintptr(menu), 0, 0x400|0x80)
			menuError = captureNativeWindow(popup, filepath.Join(output, "Native_Tray_Menu.png"))
		}
		user32.NewProc("EndMenu").Call()
	}
	if timer, _, nativeError := user32.NewProc("SetTimer").Call(hwnd, 0x7201, 200, 0); timer == 0 {
		return report, fmtNative("无法启动托盘菜单自检定时器", nativeError)
	}
	defer user32.NewProc("KillTimer").Call(hwnd, 0x7201)
	menuRect := w32.GetWindowRect(w32.HWND(hwnd))
	if menuRect == nil {
		return report, errors.New("无法读取托盘菜单自检窗口位置")
	}
	w32.TrackPopupMenuEx(menu, 0, menuRect.Left+40, menuRect.Top+120, w32.HWND(hwnd), nil)
	if menuError != nil {
		return report, menuError
	}
	check(menuShown && testTheme.menu == 0, "托盘菜单打开/选中行渲染/关闭与状态清理")
	if highContrast() {
		report.Notes = append(report.Notes, "高对比度已启用，托盘菜单按产品策略使用 Windows 系统无障碍绘制。")
	} else {
		check(menuDrawn, "托盘菜单实际执行内置 Eden UI 字体的 owner-draw 绘制")
	}
	report.Screenshots = append(report.Screenshots, "Native_Tray_Menu.png")
	testTheme = nil
	menuTimer = nil
	for _, test := range []struct {
		confirmation bool
		key          uintptr
		long         bool
	}{{false, 13, false}, {true, 13, false}, {true, 27, false}, {true, 27, true}} {
		message := "Claude Eden 2186\n请先保存编辑器中的文件并结束正在运行的工作。\n中文繁體日本語 Français Deutsch。"
		if test.long {
			message = strings.Repeat(message+"\n", 30)
		}
		var probeError error
		name := fmt.Sprintf("Native_Dialog_%t_%d_%t.png", test.confirmation, test.key, test.long)
		accepted, dialogError := showNativeDialog(hwnd, nativeDialogOptions{title: "Claude Eden 原生对话框自检", message: message, confirmation: test.confirmation, fonts: fonts, timer: func(dialog uintptr) {
			probeError = verifySelfTestControlFonts(dialog, fonts)
			if test.long {
				probeError = errors.Join(probeError, verifySelfTestDialogScroll(dialog))
			}
			if captureError := captureNativeWindow(dialog, filepath.Join(output, name)); captureError != nil {
				probeError = errors.Join(probeError, captureError)
			}
			user32.NewProc("PostMessageW").Call(dialog, 0x100, test.key, 0)
		}})
		if dialogError != nil {
			return report, dialogError
		}
		if probeError != nil {
			return report, probeError
		}
		check(accepted == (test.key == 13), "原生对话框内置字体、Enter/Escape、确定/取消与长文本滚动")
		report.Screenshots = append(report.Screenshots, name)
	}

	clipboard := nativeClipboard{hwnd: hwnd}
	var saved []struct {
		format uintptr
		data   []byte
	}
	// Preserve each supported HGLOBAL format byte-for-byte, including mixed
	// text/image/file sets. The production reader deliberately picks one type.
	e = clipboard.opened(func() error {
		previous := uintptr(0)
		for {
			kernel32.NewProc("SetLastError").Call(0)
			format, _, nativeError := user32.NewProc("EnumClipboardFormats").Call(previous)
			if format == 0 {
				if nativeError != syscall.Errno(0) {
					return fmtNative("无法完整保存 VM 原剪贴板格式", nativeError)
				}
				return nil
			}
			previous = format
			if format != 1 && format != 7 && format != 8 && format != 13 && format != 15 && format != 16 && format != 17 && format != pngFormat() {
				return fmt.Errorf("VM 剪贴板含不可还原格式 %d，请先在 VM 内清空或保存剪贴板后重跑", format)
			}
			data, err := clipboardBytes(format, maxPixels*4+4096)
			if err != nil {
				return err
			}
			saved = append(saved, struct {
				format uintptr
				data   []byte
			}{format, data})
		}
	})
	if e != nil {
		return report, e
	}
	defer func() {
		restoreError := clipboard.opened(func() error {
			if result, _, nativeError := emptyClipboard.Call(); result == 0 {
				return fmtNative("无法清空 VM 测试剪贴板", nativeError)
			}
			for _, item := range saved {
				if err := putClipboard(item.format, item.data); err != nil {
					return err
				}
			}
			return nil
		})
		if restoreError != nil {
			report.Errors = append(report.Errors, "恢复 VM 剪贴板："+restoreError.Error())
		}
	}()
	_, _, e = clipboard.write(clipboardSnapshot{text: "Claude Eden 中文 😀"}, nil)
	check(e == nil, "VM 剪贴板 Unicode 写入")
	read, e := clipboard.read()
	check(e == nil && read.text == "Claude Eden 中文 😀", "VM 剪贴板 Unicode 回读")
	img := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	img.SetNRGBA(4, 3, color.NRGBA{R: 120, G: 40, B: 90, A: 255})
	var imageData bytes.Buffer
	_ = png.Encode(&imageData, img)
	_, _, e = clipboard.write(clipboardSnapshot{png: imageData.Bytes()}, nil)
	check(e == nil, "VM 剪贴板 PNG/DIB 双格式写入")
	read, e = clipboard.read()
	check(e == nil && bytes.Equal(read.png, imageData.Bytes()), "VM 剪贴板 PNG 回读")
	var mixedText []byte
	for _, unit := range windows.StringToUTF16("图片附带的 Unicode 文字 😀") {
		mixedText = binary.LittleEndian.AppendUint16(mixedText, unit)
	}
	e = clipboard.opened(func() error { return putClipboard(13, mixedText) })
	check(e == nil, "VM 剪贴板同时发布图片与 Unicode 文字")
	text, textError := clipboard.readText()
	read, e = clipboard.read()
	check(textError == nil && text == "图片附带的 Unicode 文字 😀" && e == nil && bytes.Equal(read.png, imageData.Bytes()), "纯文本读取独立于图片优先级，富内容读取仍选择图片")
	files := []string{filepath.Join(output, "Clipboard_File_One.txt"), filepath.Join(output, "Clipboard_File_Two.txt")}
	for _, name := range files {
		if e = os.WriteFile(name, []byte("VM clipboard self-test\n"), 0600); e != nil {
			return report, e
		}
	}
	_, _, e = clipboard.write(clipboardSnapshot{paths: files}, nil)
	check(e == nil, "VM 剪贴板 CF_HDROP 写入")
	read, e = clipboard.read()
	check(e == nil && len(read.paths) == 2 && read.paths[0] == files[0] && read.paths[1] == files[1], "VM 剪贴板 CF_HDROP 回读")
	old := clipboard.revision() - 1
	_, superseded, e := clipboard.write(clipboardSnapshot{text: "must-not-write"}, &old)
	check(e == nil && superseded, "VM 剪贴板旧 revision 拒绝覆盖")
	_, e = input.request(RemoteInputRequest{Owner: owner, Operation: "deactivate", ReturnFocus: true}, 1)
	check(e == nil, "原生输入释放与生命周期")
	// Close the Win32 fixture before Wails owns the application message loop.
	input.close()
	showWindow.Call(hwnd, 0)
	if webview {
		if e = runWebviewSelfTest(output, &report, options, fonts); e != nil {
			return report, e
		}
	}
	return report, nil
}

func verifySelfTestDialogScroll(dialog uintptr) error {
	textWindow, _, _ := user32.NewProc("GetDlgItem").Call(dialog, 0)
	if textWindow == 0 {
		return errors.New("长文本对话框缺少可滚动文本控件")
	}
	var scroll struct {
		Size, Mask      uint32
		Min, Max        int32
		Page            uint32
		Position, Track int32
	}
	scroll.Size, scroll.Mask = uint32(unsafe.Sizeof(scroll)), 0x17
	result, _, _ := user32.NewProc("GetScrollInfo").Call(textWindow, 1, uintptr(unsafe.Pointer(&scroll)))
	if result == 0 || scroll.Max <= int32(scroll.Page) {
		return errors.New("长文本对话框没有完整滚动范围")
	}
	sendMessage.Call(textWindow, 0x20a, uintptr(uint32(0xff880000)), 0)
	position, _, _ := user32.NewProc("GetScrollPos").Call(textWindow, 1)
	if position == 0 {
		return errors.New("长文本对话框未响应滚轮")
	}
	return nil
}

func verifySelfTestStatusButtons(status *nativeStatus, picture string) error {
	file, err := os.Open(picture)
	if err != nil {
		return err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return err
	}
	var parent winRect
	user32.NewProc("GetWindowRect").Call(status.parent, uintptr(unsafe.Pointer(&parent)))
	for _, button := range []uintptr{status.retry, status.existing, status.complete} {
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(button)
		if visible == 0 {
			continue
		}
		var bounds winRect
		user32.NewProc("GetWindowRect").Call(button, uintptr(unsafe.Pointer(&bounds)))
		area := image.Rect(int(bounds.Left-parent.Left)+8, int(bounds.Top-parent.Top)+8, int(bounds.Right-parent.Left)-8, int(bounds.Bottom-parent.Top)-8).Intersect(img.Bounds())
		colors, largest, total := make(map[color.Color]int), 0, 0
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				pixel := img.At(x, y)
				colors[pixel]++
				largest = max(largest, colors[pixel])
				total++
			}
		}
		if total-largest < 16 {
			return errors.New("启动状态按钮正文未实际绘制，不能仅凭字体句柄判为通过")
		}
	}
	return nil
}

func verifySelfTestDialogBody(dialog uintptr, picture string) error {
	file, err := os.Open(picture)
	if err != nil {
		return err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return err
	}
	text, _, _ := user32.NewProc("GetDlgItem").Call(dialog, 0)
	var parent, bounds winRect
	user32.NewProc("GetWindowRect").Call(dialog, uintptr(unsafe.Pointer(&parent)))
	user32.NewProc("GetWindowRect").Call(text, uintptr(unsafe.Pointer(&bounds)))
	area := image.Rect(int(bounds.Left-parent.Left)+2, int(bounds.Top-parent.Top)+2, int(bounds.Right-parent.Left)-2, int(bounds.Bottom-parent.Top)-2).Intersect(img.Bounds())
	if area.Empty() {
		return errors.New("启动失败对话框正文没有可回读的实际屏幕区域")
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		dark := 0
		for x := area.Min.X; x < area.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if max(pixel.R, pixel.G, pixel.B) < 80 {
				dark++
			}
		}
		if dark*5 >= area.Dx()*4 {
			return errors.New("启动失败对话框正文区域出现横贯黑条，真实屏幕绘制验收未通过")
		}
	}
	return nil
}

func verifySelfTestControlFonts(parent uintptr, fonts *nativeFonts) error {
	if fonts == nil || fonts.family != "Eden UI" || len(fonts.registrations) != 2 {
		return errors.New("VM 自检控件没有加载两份内置 Eden UI 字体")
	}
	var fontError error
	controls := 0
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		controls++
		font, _, _ := sendMessage.Call(hwnd, 0x31, 0, 0)
		owned := false
		for _, handle := range fonts.handles {
			if font != 0 && font == uintptr(handle) {
				owned = true
				break
			}
		}
		if !owned {
			fontError = fmt.Errorf("原生控件 0x%x 未使用本次注册的内置字体", hwnd)
			return 0
		}
		if err := verifyNativeFont(w32.HFONT(font)); err != nil {
			fontError = err
			return 0
		}
		return 1
	})
	user32.NewProc("EnumChildWindows").Call(parent, callback, 0)
	if fontError != nil {
		return fontError
	}
	if controls == 0 {
		return errors.New("原生字体验收没有枚举到实际控件")
	}
	return nil
}

func pumpNativeMessages(duration time.Duration) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		var message w32.MSG
		for w32.PeekMessage(&message, 0, 0, 0, 1) {
			w32.TranslateMessage(&message)
			w32.DispatchMessage(&message)
		}
		time.Sleep(time.Millisecond)
	}
}

func exerciseIMMComposition(input *nativeInput, report *NativeSelfTestReport) error {
	context, _, _ := immGetContext.Call(input.hwnd)
	if context == 0 {
		return errors.New("未取得 IMM 组合上下文；需交互输入法验收")
	}
	defer immReleaseContext.Call(input.hwnd, context)
	text := []uint16{'测', '试'}
	result, _, _ := imm32.NewProc("ImmSetCompositionStringW").Call(context, 9, uintptr(unsafe.Pointer(&text[0])), 4, 0, 0)
	if result == 0 {
		return errors.New("当前 TSF/IMM 输入法拒绝程序化组合文本；compositionupdate/RESULTSTR 必须通过 VM 实际输入法选词验收")
	}
	start := len(report.InputEvents)
	sendMessage.Call(input.hwnd, 0x10f, 0, 8|0x80)
	if len(report.InputEvents) <= start || report.InputEvents[len(report.InputEvents)-1].Text != "测试" {
		return errors.New("程序化 IMM 组合未返回预期文本，保留实际输入法验收缺口")
	}
	report.Checks = append(report.Checks, "真实 IMM 组合文本回读 compositionupdate")
	input.cancel()
	return nil
}

func captureNativeWindow(hwnd uintptr, path string) error {
	return captureNativeWindowImage(hwnd, path, false)
}

// VM-only independent readback of the visible owned window. PrintWindow may
// omit composited controls; screen pixels distinguish that limitation from a
// real missing label. Capture never includes the rest of the guest desktop.
func captureVisibleNativeWindow(hwnd uintptr, path string) error {
	if _, err := nativeTestMachine(); err != nil {
		return err
	}
	foreground, _, _ := getForegroundWindow.Call()
	if foreground != hwnd {
		return errors.New("可见窗口截图拒绝读取非前台测试窗口")
	}
	return captureNativeWindowImage(hwnd, path, true)
}

func captureNativeWindowImage(hwnd uintptr, path string, visible bool) error {
	rect := w32.GetWindowRect(w32.HWND(hwnd))
	if rect == nil {
		return errors.New("无法读取截图窗口尺寸")
	}
	width, height := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	if width <= 0 || height <= 0 || int64(width)*int64(height) > maxPixels {
		return errors.New("截图尺寸无效")
	}
	var dc w32.HDC
	if visible {
		user32.NewProc("RedrawWindow").Call(hwnd, 0, 0, 0x0185) // INVALIDATE|ERASE|ALLCHILDREN|UPDATENOW.
		windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmFlush").Call()
		dc = w32.GetDC(0)
	} else {
		dc = w32.GetDC(w32.HWND(hwnd))
	}
	if dc == 0 {
		return errors.New("无法读取窗口绘图上下文")
	}
	defer func() {
		if visible {
			w32.ReleaseDC(0, dc)
			return
		}
		w32.ReleaseDC(w32.HWND(hwnd), dc)
	}()
	memory := w32.CreateCompatibleDC(dc)
	if memory == 0 {
		return errors.New("无法创建截图绘图上下文")
	}
	defer w32.DeleteDC(memory)
	// A top-down 32-bit DIB is allocated by GDI; only this window is rendered,
	// not the screen or other applications' content.
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header, 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(width))
	binary.LittleEndian.PutUint32(header[8:], uint32(int32(-height)))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	var bits unsafe.Pointer
	bitmap := w32.CreateDIBSection(dc, (*w32.BITMAPINFO)(unsafe.Pointer(&header[0])), 0, &bits, 0, 0)
	if bitmap == 0 || bits == nil {
		return errors.New("无法分配截图位图")
	}
	defer w32.DeleteObject(w32.HGDIOBJ(bitmap))
	previous := w32.SelectObject(memory, w32.HGDIOBJ(bitmap))
	defer w32.SelectObject(memory, previous)
	var result uintptr
	if visible {
		result, _, _ = windowsGDI.NewProc("BitBlt").Call(uintptr(memory), 0, 0, uintptr(width), uintptr(height), uintptr(dc), uintptr(rect.Left), uintptr(rect.Top), 0x00cc0020)
	} else {
		result, _, _ = user32.NewProc("PrintWindow").Call(hwnd, uintptr(memory), 2)
	}
	if result == 0 {
		return errors.New("窗口截图失败")
	}
	data := unsafe.Slice((*byte)(bits), width*height*4)
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for index := 0; index < len(data); index += 4 {
		img.Pix[index], img.Pix[index+1], img.Pix[index+2], img.Pix[index+3] = data[index+2], data[index+1], data[index], 255
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}
