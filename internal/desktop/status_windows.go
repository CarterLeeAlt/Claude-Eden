package desktop

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type nativeStatus struct {
	parent, panel, text, retry, existing, complete uintptr
	quit                                           bool
	fonts                                          *nativeFonts
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
	status := &nativeStatus{parent: parent, fonts: fonts}
	var err error
	status.panel, err = create("STATIC", "", 0x52000000, parent, 0)
	if err != nil {
		return nil, err
	}
	status.text, err = create("STATIC", "正在启动 Claude Eden 工作台…", 0x50000000, status.panel, 0)
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
	x := max(unit(28), (width-unit(720))/2)
	y := max(unit(32), (height-unit(300))/2)
	setWindowPos.Call(s.panel, 0, 0, 0, uintptr(width), uintptr(height), 0x0010)
	setWindowPos.Call(s.text, 0, uintptr(x), uintptr(y), uintptr(max(unit(100), width-2*x)), uintptr(unit(160)), 0x0010)
	setWindowPos.Call(s.retry, 0, uintptr(x), uintptr(y+unit(180)), uintptr(unit(140)), uintptr(unit(36)), 0x0010)
	setWindowPos.Call(s.existing, 0, uintptr(x), uintptr(y+unit(180)), uintptr(unit(230)), uintptr(unit(36)), 0x0010)
	setWindowPos.Call(s.complete, 0, uintptr(x+unit(244)), uintptr(y+unit(180)), uintptr(unit(280)), uintptr(unit(36)), 0x0010)
}

func (s *nativeStatus) show(message string, retry, upgrade bool) {
	if s == nil {
		return
	}
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
	for _, window := range []uintptr{s.panel, s.retry, s.existing, s.complete} {
		if window != 0 {
			destroyWindow.Call(window)
		}
	}
	s.panel, s.text, s.retry, s.existing, s.complete = 0, 0, 0, 0, 0
}
