package desktop

import (
	"errors"
	"math"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

type nativeFonts struct {
	registrations []uintptr
	handles       map[int]w32.HFONT
	family        string
}

func newNativeFonts(regular, bold []byte) (*nativeFonts, error) {
	if len(regular) == 0 || len(bold) == 0 {
		return nil, errors.New("缺少内置界面字体，请重新安装 Claude Eden")
	}
	fonts := &nativeFonts{handles: make(map[int]w32.HFONT), family: "Eden UI"}
	for _, data := range [][]byte{regular, bold} {
		var count uint32
		handle, _, err := windowsGDI.NewProc("AddFontMemResourceEx").Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0, uintptr(unsafe.Pointer(&count)))
		if handle == 0 || count == 0 {
			fonts.close()
			return nil, fmtNative("无法加载内置界面字体", err)
		}
		fonts.registrations = append(fonts.registrations, handle)
	}
	for _, bold := range []bool{false, true} {
		if err := verifyNativeFont(fonts.font(96, bold)); err != nil {
			fonts.close()
			return nil, err
		}
	}
	return fonts, nil
}

func verifyNativeFont(font w32.HFONT) error {
	if font == 0 {
		return errors.New("缺少内置界面字体句柄")
	}
	dc := w32.CreateCompatibleDC(0)
	if dc == 0 {
		return errors.New("无法核对内置界面字体")
	}
	defer w32.DeleteDC(dc)
	previous := w32.SelectObject(dc, w32.HGDIOBJ(font))
	defer w32.SelectObject(dc, previous)
	name := make([]uint16, 64)
	count, _, _ := windowsGDI.NewProc("GetTextFaceW").Call(uintptr(dc), uintptr(len(name)), uintptr(unsafe.Pointer(&name[0])))
	if count == 0 || windows.UTF16ToString(name) != "Eden UI" {
		return errors.New("GDI 未使用内置 Eden UI 字体，请重新安装 Claude Eden")
	}
	text := windows.StringToUTF16("Claude Eden 2186 中文繁體日本語 Français Deutsch")
	glyphs := make([]uint16, len(text)-1)
	result, _, _ := windowsGDI.NewProc("GetGlyphIndicesW").Call(uintptr(dc), uintptr(unsafe.Pointer(&text[0])), uintptr(len(glyphs)), uintptr(unsafe.Pointer(&glyphs[0])), 1)
	if uint32(result) == 0xffffffff {
		return errors.New("无法核对内置界面字体字形")
	}
	for _, glyph := range glyphs {
		if glyph == 0xffff {
			return errors.New("内置界面字体缺少必需字形，请重新安装 Claude Eden")
		}
	}
	return nil
}

func (fonts *nativeFonts) font(dpi int, bold bool) w32.HFONT {
	if dpi <= 0 {
		dpi = 96
	}
	key := dpi * 2
	if bold {
		key++
	}
	if handle := fonts.handles[key]; handle != 0 {
		return handle
	}
	var value w32.LOGFONT
	value.Height = -int32(math.Round(14 * float64(dpi) / 96))
	value.Weight = 400
	if bold {
		value.Weight = 700
	}
	value.CharSet = 1
	value.Quality = 5 // DEFAULT_CHARSET, ClearType.
	for index, unit := range []rune(fonts.family) {
		if index >= len(value.FaceName)-1 {
			break
		}
		value.FaceName[index] = uint16(unit)
	}
	handle := w32.CreateFontIndirect(&value)
	fonts.handles[key] = handle
	return handle
}

func (fonts *nativeFonts) close() {
	if fonts == nil {
		return
	}
	for _, handle := range fonts.handles {
		if handle != 0 {
			w32.DeleteObject(w32.HGDIOBJ(handle))
		}
	}
	for _, handle := range fonts.registrations {
		windowsGDI.NewProc("RemoveFontMemResourceEx").Call(handle)
	}
	fonts.registrations = nil
	fonts.handles = nil
}
