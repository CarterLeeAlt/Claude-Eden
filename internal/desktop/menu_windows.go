package desktop

import (
	"math"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

type nativeMenuEntry struct {
	caption      string
	originalType uint32
	originalData uintptr
	separator    bool
}
type nativeMenuTheme struct {
	fonts                  *nativeFonts
	background, foreground application.RGBA
	menu                   w32.HMENU
	entries                map[uintptr]nativeMenuEntry
	dpi                    int
}

func colourRef(value application.RGBA) uint32 {
	return uint32(value.Red) | uint32(value.Green)<<8 | uint32(value.Blue)<<16
}
func menuHighlight(fill, ink application.RGBA) application.RGBA {
	return application.NewRGB(uint8((uint16(fill.Red)*4+uint16(ink.Red))/5), uint8((uint16(fill.Green)*4+uint16(ink.Green))/5), uint8((uint16(fill.Blue)*4+uint16(ink.Blue))/5))
}

func highContrast() bool {
	value := struct {
		Size, Flags uint32
		Scheme      uintptr
	}{}
	value.Size = uint32(unsafe.Sizeof(value))
	result, _, _ := user32.NewProc("SystemParametersInfoW").Call(0x42, uintptr(value.Size), uintptr(unsafe.Pointer(&value)), 0)
	return result != 0 && value.Flags&1 != 0
}

func (t *nativeMenuTheme) unit(value int) int {
	return int(math.Round(float64(value) * float64(t.dpi) / 96))
}
func (t *nativeMenuTheme) prepare(menu w32.HMENU) bool {
	if highContrast() {
		return false
	}
	count, _, _ := user32.NewProc("GetMenuItemCount").Call(uintptr(menu))
	if count != 5 {
		return false
	}
	var entries []nativeMenuEntry
	for index := uint32(0); index < uint32(count); index++ {
		buffer := make([]uint16, 512)
		item := w32.MENUITEMINFO{FMask: 0x100 | 0x40 | 0x20, DwTypeData: &buffer[0], Cch: uint32(len(buffer))}
		item.CbSize = uint32(unsafe.Sizeof(item))
		if !w32.GetMenuItemInfo(menu, index, true, &item) {
			return false
		}
		entries = append(entries, nativeMenuEntry{windows.UTF16ToString(buffer), item.FType, item.DwItemData, item.FType&0x800 != 0})
	}
	if entries[0].caption != "打开 Claude Eden" || entries[1].caption != "检查应用更新…" || entries[2].caption != "关于 Claude Eden" || !entries[3].separator || entries[4].caption != "退出 Claude Eden…" {
		return false
	}
	t.menu = menu
	t.entries = make(map[uintptr]nativeMenuEntry)
	dpi := uint32(96)
	var point winPoint
	user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
	packed := uintptr(uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32)
	monitor, _, _ := user32.NewProc("MonitorFromPoint").Call(packed, 2)
	var y uint32
	if proc := windows.NewLazySystemDLL("shcore.dll").NewProc("GetDpiForMonitor"); proc.Find() == nil {
		proc.Call(monitor, 0, uintptr(unsafe.Pointer(&dpi)), uintptr(unsafe.Pointer(&y)))
	}
	if dpi == 0 {
		dpi = 96
	}
	t.dpi = int(dpi)
	for index, entry := range entries {
		token := uintptr(0x45440000 + index)
		t.entries[token] = entry
		item := w32.MENUITEMINFO{FMask: 0x100 | 0x20, FType: entry.originalType | 0x100, DwItemData: token}
		item.CbSize = uint32(unsafe.Sizeof(item))
		if !w32.SetMenuItemInfo(menu, uint32(index), true, &item) {
			t.reset()
			return false
		}
	}
	return true
}

func (t *nativeMenuTheme) reset() {
	for token, entry := range t.entries {
		item := w32.MENUITEMINFO{FMask: 0x100 | 0x20, FType: entry.originalType, DwItemData: entry.originalData}
		item.CbSize = uint32(unsafe.Sizeof(item))
		w32.SetMenuItemInfo(t.menu, uint32(token-0x45440000), true, &item)
	}
	t.entries = nil
	t.menu = 0
}

func (t *nativeMenuTheme) hook(hwnd uintptr, message uint32, wParam, lParam uintptr) (uintptr, bool) {
	if t == nil {
		return 0, false
	}
	switch message {
	case 0x0117:
		t.prepare(w32.HMENU(wParam))
	case 0x0125:
		if w32.HMENU(wParam) == t.menu {
			t.reset()
		}
	case 0x002c:
		if lParam == 0 {
			return 0, false
		}
		value := (*w32.MEASUREITEMSTRUCT)(unsafe.Pointer(lParam))
		entry, ok := t.entries[value.ItemData]
		if !ok || value.CtlType != 1 {
			return 0, false
		}
		if entry.separator {
			value.ItemHeight = uint32(t.unit(9))
			value.ItemWidth = uint32(t.unit(208))
			return 1, true
		}
		dc := w32.GetDC(w32.HWND(hwnd))
		previous := w32.SelectObject(dc, w32.HGDIOBJ(t.fonts.font(t.dpi, false)))
		text := windows.StringToUTF16(entry.caption)
		var size w32.SIZE
		w32.GetTextExtentPoint32(dc, &text[0], len(text)-1, &size)
		w32.SelectObject(dc, previous)
		w32.ReleaseDC(w32.HWND(hwnd), dc)
		value.ItemWidth = uint32(max(t.unit(208), int(size.CX)+t.unit(24)))
		value.ItemHeight = uint32(int(size.CY) + t.unit(14))
		return 1, true
	case 0x002b:
		if lParam == 0 {
			return 0, false
		}
		value := (*w32.DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		entry, ok := t.entries[value.ItemData]
		if !ok || value.ControlType != 1 {
			return 0, false
		}
		brush, _, _ := windowsGDI.NewProc("CreateSolidBrush").Call(uintptr(colourRef(t.background)))
		w32.FillRect(value.HDC, &value.RcItem, w32.HBRUSH(brush))
		w32.DeleteObject(w32.HGDIOBJ(brush))
		if value.ItemState&1 != 0 && !entry.separator {
			selected, _, _ := windowsGDI.NewProc("CreateSolidBrush").Call(uintptr(colourRef(menuHighlight(t.background, t.foreground))))
			previousBrush := w32.SelectObject(value.HDC, w32.HGDIOBJ(selected))
			nullPen, _, _ := windowsGDI.NewProc("GetStockObject").Call(8)
			previousPen := w32.SelectObject(value.HDC, w32.HGDIOBJ(nullPen))
			windowsGDI.NewProc("RoundRect").Call(uintptr(value.HDC), uintptr(value.RcItem.Left+int32(t.unit(2))), uintptr(value.RcItem.Top+int32(t.unit(1))), uintptr(value.RcItem.Right-int32(t.unit(2))), uintptr(value.RcItem.Bottom-int32(t.unit(1))), uintptr(t.unit(5)), uintptr(t.unit(5)))
			w32.SelectObject(value.HDC, previousPen)
			w32.SelectObject(value.HDC, previousBrush)
			w32.DeleteObject(w32.HGDIOBJ(selected))
		}
		if entry.separator {
			line := value.RcItem
			line.Left += int32(t.unit(10))
			line.Right -= int32(t.unit(10))
			line.Top = (line.Top + line.Bottom) / 2
			line.Bottom = line.Top + 1
			brush, _, _ := windowsGDI.NewProc("CreateSolidBrush").Call(uintptr(colourRef(menuHighlight(t.background, t.foreground))))
			w32.FillRect(value.HDC, &line, w32.HBRUSH(brush))
			w32.DeleteObject(w32.HGDIOBJ(brush))
			return 1, true
		}
		previous := w32.SelectObject(value.HDC, w32.HGDIOBJ(t.fonts.font(t.dpi, false)))
		defer w32.SelectObject(value.HDC, previous)
		windowsGDI.NewProc("SetBkMode").Call(uintptr(value.HDC), 1)
		w32.SetTextColor(value.HDC, w32.COLORREF(colourRef(t.foreground)))
		text := windows.StringToUTF16(entry.caption)
		rect := value.RcItem
		rect.Left += int32(t.unit(12))
		rect.Right -= int32(t.unit(12))
		w32.DrawText(value.HDC, text, len(text)-1, &rect, 0x20|4|0x800)
		return 1, true
	}
	return 0, false
}
