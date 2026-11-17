package desktop

import (
	"errors"
	"sync"
	"syscall"
	"unsafe"

	"github.com/sylvinus/agent-vm/internal/uitheme"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

type nativeStatus struct {
	parent, panel, text, retry, existing, complete uintptr
	quit                                           bool
	fonts                                          *nativeFonts
	message                                        string
	retrying, upgrading                            bool
	brush, foreground                              uintptr
}

var statusPanels sync.Map
var statusPanelCallback = syscall.NewCallback(func(hwnd uintptr, message uint32, wParam, lParam, id, data uintptr) uintptr {
	if value, ok := statusPanels.Load(hwnd); ok {
		s := value.(*nativeStatus)
		if result, handled := s.colour(message, wParam, lParam); handled {
			return result
		}
	}
	result, _, _ := defSubclassProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
})

func (s *nativeStatus) colour(message uint32, dc, control uintptr) (uintptr, bool) {
	if message != 0x0138 || (control != s.panel && control != s.text) || highContrast() {
		return 0, false
	}
	windowsGDI.NewProc("SetTextColor").Call(dc, s.foreground)
	windowsGDI.NewProc("SetBkMode").Call(dc, 1)
	return s.brush, true
}

func newNativeStatus(parent uintptr, fonts *nativeFonts) (*nativeStatus, error) {
	if fonts == nil {
		return nil, errors.New("启动状态控件缺少内置界面字体")
	}
	dpi, _, _ := getDPIForWindow.Call(parent)
	create := func(class, text string, style uintptr, owner, id uintptr) (uintptr, error) {
		// The panel must not repaint over sibling buttons, nor over its text child.
		window, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(class))), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(text))), style|0x04000000, 0, 0, 1, 1, owner, id, 0, 0)
		if window == 0 {
			return 0, fmtNative("无法创建启动状态控件", err)
		}
		sendMessage.Call(window, 0x30, uintptr(fonts.font(int(dpi), false)), 1)
		return window, nil
	}
	status := &nativeStatus{parent: parent, fonts: fonts, message: "正在启动 ClaudeEden 工作台…"}
	palette := uitheme.Default()
	background, err := parseColour(palette.Surface)
	if err != nil {
		return nil, err
	}
	foreground, err := parseColour(palette.Ink)
	if err != nil {
		return nil, err
	}
	status.foreground = uintptr(colourRef(foreground))
	status.brush, _, err = windowsGDI.NewProc("CreateSolidBrush").Call(uintptr(colourRef(background)))
	if status.brush == 0 {
		return nil, fmtNative("无法创建启动背景", err)
	}
	status.panel, err = create("STATIC", "", 0x52000000, parent, 0)
	if err != nil {
		status.close()
		return nil, err
	}
	statusPanels.Store(status.panel, status)
	if result, _, nativeErr := setWindowSubclass.Call(status.panel, statusPanelCallback, 0x7100, 0); result == 0 {
		status.close()
		return nil, fmtNative("无法统一启动提示背景", nativeErr)
	}
	// SS_CENTER | SS_NOPREFIX: center the actual line, not just its container.
	status.text, err = create("STATIC", status.message, 0x50000081, status.panel, 0)
	if err != nil {
		status.close()
		return nil, err
	}
	// Buttons are direct children of the main HWND so WndProcInterceptor receives
	// WM_COMMAND. Their IDs are local to the trusted native startup controls.
	status.retry, err = create("BUTTON", "重试连接", 0x40010000, parent, 0x7101)
	if err != nil {
		status.close()
		return nil, err
	}
	status.existing, err = create("BUTTON", "打开现有环境保存文件", 0x40010000, parent, 0x7102)
	if err != nil {
		status.close()
		return nil, err
	}
	status.complete, err = create("BUTTON", "保存后停止环境并完成更新", 0x40010000, parent, 0x7103)
	if err != nil {
		status.close()
		return nil, err
	}
	status.layout()
	return status, nil
}

func (s *nativeStatus) layout() {
	if s == nil || s.panel == 0 {
		return
	}
	var rect winRect
	user32.NewProc("GetClientRect").Call(s.parent, uintptr(unsafe.Pointer(&rect)))
	dpi, _, _ := getDPIForWindow.Call(s.parent)
	if dpi == 0 {
		dpi = 96
	}
	unit := func(value int32) int32 { return value * int32(dpi) / 96 }
	for _, window := range []uintptr{s.panel, s.text, s.retry, s.existing, s.complete} {
		sendMessage.Call(window, 0x30, uintptr(s.fonts.font(int(dpi), false)), 1)
	}
	width, height := rect.Right-rect.Left, rect.Bottom-rect.Top
	textWidth := max(1, min(unit(720), width-unit(56)))
	text := windows.StringToUTF16(s.message)
	dc := w32.GetDC(w32.HWND(s.parent))
	previous := w32.SelectObject(dc, w32.HGDIOBJ(s.fonts.font(int(dpi), false)))
	textRect := w32.RECT{Right: textWidth}
	w32.DrawText(dc, text, len(text)-1, &textRect, 0x0400|0x0010|0x0800)
	w32.SelectObject(dc, previous)
	w32.ReleaseDC(w32.HWND(s.parent), dc)
	textHeight := max(unit(24), textRect.Bottom-textRect.Top)
	textBounds, retryBounds, existingBounds, completeBounds := statusControlBounds(width, height, int32(dpi), textHeight, s.retrying, s.upgrading)
	setWindowPos.Call(s.panel, 0, 0, 0, uintptr(width), uintptr(height), 0x0010)
	for _, control := range []struct {
		handle uintptr
		bounds winRect
	}{{s.text, textBounds}, {s.retry, retryBounds}, {s.existing, existingBounds}, {s.complete, completeBounds}} {
		b := control.bounds
		setWindowPos.Call(control.handle, 0, uintptr(b.Left), uintptr(b.Top), uintptr(b.Right-b.Left), uintptr(b.Bottom-b.Top), 0x0010)
	}
}

// All coordinates are physical client pixels. Only visible actions reserve
// space; a one-line pending message is centered on its own, at every DPI/size.
func statusControlBounds(width, height, dpi, textHeight int32, retry, upgrade bool) (text, retryButton, existing, complete winRect) {
	unit := func(value int32) int32 { return value * dpi / 96 }
	textWidth := max(1, min(unit(720), width-unit(56)))
	contentHeight := textHeight
	if retry || upgrade {
		contentHeight += unit(24 + 36)
	}
	x, y := (width-textWidth)/2, max(unit(28), (height-contentHeight)/2)
	text = winRect{Left: x, Top: y, Right: x + textWidth, Bottom: y + textHeight}
	buttonY := text.Bottom + unit(24)
	button := func(x, w int32) winRect {
		return winRect{Left: x, Top: buttonY, Right: x + w, Bottom: buttonY + unit(36)}
	}
	retryButton = button((width-unit(140))/2, unit(140))
	actionX := (width - unit(230+12+280)) / 2
	existing = button(actionX, unit(230))
	complete = button(actionX+unit(230+12), unit(280))
	return
}

func (s *nativeStatus) show(message string, retry, upgrade bool) {
	if s == nil {
		return
	}
	s.message, s.retrying, s.upgrading = message, retry, upgrade
	sendMessage.Call(s.text, 0x000c, 0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(message))))
	showWindow.Call(s.panel, 5)
	setWindowPos.Call(s.panel, 0, 0, 0, 0, 0, 0x0013)
	for _, button := range []struct {
		handle  uintptr
		visible bool
	}{{s.retry, retry}, {s.existing, upgrade}, {s.complete, upgrade}} {
		command := uintptr(0)
		if button.visible {
			command = 5
		}
		showWindow.Call(button.handle, command)
		if button.visible {
			setWindowPos.Call(button.handle, 0, 0, 0, 0, 0, 0x0013)
		}
	}
	s.layout()
}

func (s *nativeStatus) hide() {
	if s == nil {
		return
	}
	for _, window := range []uintptr{s.panel, s.retry, s.existing, s.complete} {
		showWindow.Call(window, 0)
	}
}
func (s *nativeStatus) close() {
	if s == nil {
		return
	}
	if s.panel != 0 {
		removeWindowSubclass.Call(s.panel, statusPanelCallback, 0x7100)
		statusPanels.Delete(s.panel)
	}
	for _, window := range []uintptr{s.panel, s.retry, s.existing, s.complete} {
		if window != 0 {
			destroyWindow.Call(window)
		}
	}
	s.panel, s.text, s.retry, s.existing, s.complete = 0, 0, 0, 0, 0
	if s.brush != 0 {
		windowsGDI.NewProc("DeleteObject").Call(s.brush)
		s.brush = 0
	}
}
