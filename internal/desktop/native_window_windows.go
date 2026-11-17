package desktop

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

func securityAttributesSize() uintptr { return unsafe.Sizeof(windows.SecurityAttributes{}) }

var (
	monitorFromWindow     = user32.NewProc("MonitorFromWindow")
	getMonitorInfo        = user32.NewProc("GetMonitorInfoW")
	getWindowPlacement    = user32.NewProc("GetWindowPlacement")
	dwmSetWindowAttribute = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
)

type nativePlacement struct {
	Length, Flags, Show uint32
	Minimum, Maximum    winPoint
	Normal              winRect
}

func (s *Shell) restorePlacement() error {
	hwnd := uintptr(s.window.NativeWindow())
	var monitor struct {
		Size          uint32
		Monitor, Work winRect
		Flags         uint32
	}
	monitor.Size = uint32(unsafe.Sizeof(monitor))
	handle, _, _ := monitorFromWindow.Call(hwnd, 2)
	result, _, err := getMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&monitor)))
	if result == 0 {
		return fmtNative("无法读取窗口工作区", err)
	}
	dpi, _, _ := getDPIForWindow.Call(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	var saved *WindowPlacement
	data, readErr := os.ReadFile(filepath.Join(s.backend.Root(), "window-placement.json"))
	if readErr == nil {
		_ = json.Unmarshal(data, &saved)
	}
	work := Rectangle{int(monitor.Work.Left), int(monitor.Work.Top), int(monitor.Work.Right - monitor.Work.Left), int(monitor.Work.Bottom - monitor.Work.Top)}
	bounds, minWidth, minHeight := windowBounds(work, int(dpi), saved)
	s.window.SetMinSize(int(float64(minWidth)*96/float64(dpi)), int(float64(minHeight)*96/float64(dpi)))
	s.window.SetPhysicalBounds(application.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height})
	if saved != nil && saved.Maximized {
		s.window.Maximise()
	}
	return nil
}

func (s *Shell) savePlacement() {
	if s.window == nil || s.window.NativeWindow() == nil {
		return
	}
	hwnd := uintptr(s.window.NativeWindow())
	placement := nativePlacement{Length: uint32(unsafe.Sizeof(nativePlacement{}))}
	result, _, _ := getWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&placement)))
	if result == 0 {
		return
	}
	dpi, _, _ := getDPIForWindow.Call(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	// WINDOWPLACEMENT uses workspace coordinates; convert to screen coordinates
	// before persisting, including taskbars on the left/top edge.
	var monitor struct {
		Size          uint32
		Monitor, Work winRect
		Flags         uint32
	}
	monitor.Size = uint32(unsafe.Sizeof(monitor))
	handle, _, _ := monitorFromWindow.Call(hwnd, 2)
	getMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&monitor)))
	x := placement.Normal.Left + monitor.Work.Left - monitor.Monitor.Left
	y := placement.Normal.Top + monitor.Work.Top - monitor.Monitor.Top
	value := WindowPlacement{Rectangle: Rectangle{int(x), int(y), int(placement.Normal.Right - placement.Normal.Left), int(placement.Normal.Bottom - placement.Normal.Top)}, DPI: int(dpi), Maximized: placement.Show == 3}
	if value.Width <= 0 || value.Height <= 0 {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	if err = os.MkdirAll(s.backend.Root(), 0700); err == nil {
		err = os.WriteFile(filepath.Join(s.backend.Root(), "window-placement.json"), data, 0600)
	}
	if err != nil {
		go s.backend.Report("Window placement: " + err.Error())
	}
}

func parseColour(value string) (application.RGBA, error) {
	if len(value) != 7 || value[0] != '#' {
		return application.RGBA{}, errors.New("外观参数无效")
	}
	data, err := hex.DecodeString(value[1:])
	if err != nil {
		return application.RGBA{}, errors.New("外观参数无效")
	}
	return application.NewRGB(data[0], data[1], data[2]), nil
}

func (s *Shell) appearance(request AppearanceRequest) error {
	background, err := parseColour(request.Background)
	if err != nil {
		return err
	}
	foreground, err := parseColour(request.Foreground)
	if err != nil {
		minimum := min(background.Red, background.Green, background.Blue)
		maximum := max(background.Red, background.Green, background.Blue)
		foreground = application.NewRGB(0, 0, 0)
		if int(minimum)+int(maximum) < 255 {
			foreground = application.NewRGB(255, 255, 255)
		}
	}
	s.window.SetBackgroundColour(background)
	return application.InvokeSyncWithError(func() error {
		if s.menuTheme != nil {
			s.menuTheme.background = background
			s.menuTheme.foreground = foreground
		}
		var light int32
		hwnd := uintptr(s.window.NativeWindow())
		// Keep Windows' high-contrast menu rendering; do not replace system menu
		// metrics with per-pixel drawing. Web content owns its explicit palette.
		dwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&light)), 4)
		return nil
	})
}

func platformSupported() error {
	if version := windows.RtlGetVersion(); version.MajorVersion < 10 {
		return errors.New("此 Claude Eden 安装包需要 Windows 10 的 x64（Intel/AMD）系统")
	}
	var processMachine, nativeMachine uint16
	result, _, err := kernel32.NewProc("IsWow64Process2").Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&processMachine)), uintptr(unsafe.Pointer(&nativeMachine)))
	if result == 0 {
		return fmtNative("无法检测 Windows 原生处理器架构", err)
	}
	if nativeMachine != 0x8664 {
		return errors.New("此 Claude Eden 安装包需要 Windows 10 的 x64（Intel/AMD）系统")
	}
	for _, name := range []string{"CreatePseudoConsole", "ResizePseudoConsole", "ClosePseudoConsole"} {
		if err := kernel32.NewProc(name).Find(); err != nil {
			return errors.New("Windows 缺少终端所需的 " + name + " 接口。请更新到提供 ConPTY 的 Windows 10 版本（微软从 1809 起提供该接口）。")
		}
	}
	return nil
}

func redactMessage(value string) string {
	words := strings.Fields(value)
	for index, word := range words {
		if offset := strings.IndexAny(word, "?#"); offset >= 0 {
			words[index] = word[:offset] + "[redacted]"
		}
	}
	result := strings.Join(words, " ")
	if len(result) > 768 {
		result = strings.ToValidUTF8(result[:768], "")
	}
	return result
}
