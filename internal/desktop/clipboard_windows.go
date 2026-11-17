package desktop

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	openClipboard           = user32.NewProc("OpenClipboard")
	closeClipboard          = user32.NewProc("CloseClipboard")
	emptyClipboard          = user32.NewProc("EmptyClipboard")
	getClipboardData        = user32.NewProc("GetClipboardData")
	setClipboardData        = user32.NewProc("SetClipboardData")
	hasClipboardFormat      = user32.NewProc("IsClipboardFormatAvailable")
	clipboardSequence       = user32.NewProc("GetClipboardSequenceNumber")
	registerClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	globalAlloc             = kernel32.NewProc("GlobalAlloc")
	globalLock              = kernel32.NewProc("GlobalLock")
	globalUnlock            = kernel32.NewProc("GlobalUnlock")
	globalSize              = kernel32.NewProc("GlobalSize")
	globalFree              = kernel32.NewProc("GlobalFree")
	dragQueryFile           = shell32.NewProc("DragQueryFileW")
)

const maxPNG = 20 << 20
const maxPixels = 40_000_000
const maxText = 2 << 20

type clipboardSnapshot struct {
	text     string
	png      []byte
	paths    []string
	revision uint32
}
type nativeClipboard struct{ hwnd uintptr }

func (c nativeClipboard) revision() uint32 {
	result, _, _ := clipboardSequence.Call()
	return uint32(result)
}
func (c nativeClipboard) opened(fn func() error) error {
	// The caller dispatches onto Wails' UI thread. Clipboard ownership and
	// Open/Close therefore remain on a single OS thread.
	result, _, err := openClipboard.Call(c.hwnd)
	if result == 0 {
		return fmtNative("无法打开剪贴板", err)
	}
	defer closeClipboard.Call()
	return fn()
}

func fmtNative(message string, err error) error {
	if err == syscall.Errno(0) {
		return errors.New(message)
	}
	return errors.New(message + "：" + err.Error())
}

func available(format uintptr) bool {
	result, _, _ := hasClipboardFormat.Call(format)
	return result != 0
}
func pngFormat() uintptr {
	result, _, _ := registerClipboardFormat.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("PNG"))))
	return result
}

func clipboardBytes(format uintptr, limit int) ([]byte, error) {
	handle, _, err := getClipboardData.Call(format)
	if handle == 0 {
		return nil, fmtNative("无法读取剪贴板", err)
	}
	size, _, _ := globalSize.Call(handle)
	if size == 0 || size > uintptr(limit) {
		return nil, errors.New("剪贴板内容超过允许大小")
	}
	pointer, _, err := globalLock.Call(handle)
	if pointer == 0 {
		return nil, fmtNative("无法锁定剪贴板内容", err)
	}
	defer globalUnlock.Call(handle)
	return bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(pointer)), int(size))), nil
}

func (c nativeClipboard) read() (snapshot clipboardSnapshot, err error) {
	err = c.opened(func() error {
		snapshot.revision = c.revision()
		if available(15) {
			handle, _, nativeErr := getClipboardData.Call(15)
			if handle == 0 {
				return fmtNative("无法读取剪贴板文件", nativeErr)
			}
			count, _, _ := dragQueryFile.Call(handle, 0xffffffff, 0, 0)
			if count == 0 || count > 100 {
				return errors.New("请选择 1 至 100 个文件")
			}
			for index := uintptr(0); index < count; index++ {
				length, _, _ := dragQueryFile.Call(handle, index, 0, 0)
				if length == 0 || length > 32767 {
					return errors.New("剪贴板文件路径无效")
				}
				buffer := make([]uint16, length+1)
				copied, _, _ := dragQueryFile.Call(handle, index, uintptr(unsafe.Pointer(&buffer[0])), length+1)
				if copied != length {
					return errors.New("无法读取剪贴板文件路径")
				}
				snapshot.paths = append(snapshot.paths, windows.UTF16ToString(buffer))
			}
			return nil
		}
		if format := pngFormat(); format != 0 && available(format) {
			data, err := clipboardBytes(format, maxPNG+64)
			if err != nil {
				return err
			}
			data, err = trimClipboardPNG(data)
			if err != nil {
				return err
			}
			if _, err = validatePNG(data); err != nil {
				return err
			}
			snapshot.png = data
			return nil
		}
		if available(8) || available(17) {
			format := uintptr(8)
			if available(17) {
				format = 17
			}
			data, err := clipboardBytes(format, maxPixels*4+4096)
			if err != nil {
				return err
			}
			snapshot.png, err = dibToPNG(data)
			return err
		}
		if available(13) {
			snapshot.text, err = clipboardText()
			return err
		}
		return nil
	})
	return
}

// Plain text reads are independent of the rich-content priority. Applications
// can publish Unicode text alongside an image or file list on the clipboard.
func (c nativeClipboard) readText() (text string, err error) {
	err = c.opened(func() error {
		if available(13) {
			text, err = clipboardText()
		}
		return err
	})
	return
}

func clipboardText() (string, error) {
	data, err := clipboardBytes(13, maxText*2+2)
	if err != nil {
		return "", err
	}
	return decodeClipboardText(data)
}

func decodeClipboardText(data []byte) (string, error) {
	if len(data)%2 != 0 {
		return "", errors.New("剪贴板文字格式无效")
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	text := windows.UTF16ToString(units)
	if len(text) > maxText {
		return "", errors.New("文本不能超过 2 MB")
	}
	return text, nil
}

func validatePNG(data []byte) (image.Image, error) {
	if len(data) > maxPNG {
		return nil, errors.New("图片不能超过 20 MB")
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("PNG 图片格式无效")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxPixels {
		return nil, errors.New("图片不能超过 4000 万像素")
	}
	return png.Decode(bytes.NewReader(data))
}

func trimClipboardPNG(data []byte) ([]byte, error) {
	if len(data) < 8 || !bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return nil, errors.New("PNG 图片格式无效")
	}
	for offset := 8; offset <= len(data)-12; {
		length := uint64(binary.BigEndian.Uint32(data[offset:]))
		end := uint64(offset) + 12 + length
		if end > uint64(len(data)) {
			break
		}
		if string(data[offset+4:offset+8]) == "IEND" && length == 0 {
			return data[:int(end)], nil
		}
		offset = int(end)
	}
	return nil, errors.New("PNG 图片内容不完整")
}

func putClipboard(format uintptr, data []byte) error {
	if len(data) == 0 {
		return errors.New("剪贴板内容为空")
	}
	handle, _, err := globalAlloc.Call(0x42, uintptr(len(data))) // GMEM_MOVEABLE|ZEROINIT
	if handle == 0 {
		return fmtNative("无法分配剪贴板内容", err)
	}
	transferred := false
	defer func() {
		if !transferred {
			globalFree.Call(handle)
		}
	}()
	pointer, _, err := globalLock.Call(handle)
	if pointer == 0 {
		return fmtNative("无法锁定剪贴板内容", err)
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(pointer)), len(data)), data)
	globalUnlock.Call(handle)
	result, _, err := setClipboardData.Call(format, handle)
	if result == 0 {
		return fmtNative("无法写入剪贴板", err)
	}
	transferred = true // Windows now owns the movable memory.
	runtime.KeepAlive(data)
	return nil
}

func (c nativeClipboard) write(snapshot clipboardSnapshot, expected *uint32) (uint32, bool, error) {
	var formats []struct {
		format uintptr
		data   []byte
	}
	if snapshot.paths != nil {
		data := make([]byte, 20)
		binary.LittleEndian.PutUint32(data, 20)
		binary.LittleEndian.PutUint32(data[16:], 1)
		for _, path := range snapshot.paths {
			units, err := windows.UTF16FromString(path)
			if err != nil {
				return 0, false, err
			}
			for _, unit := range units {
				data = binary.LittleEndian.AppendUint16(data, unit)
			}
		}
		data = append(data, 0, 0)
		formats = append(formats, struct {
			format uintptr
			data   []byte
		}{15, data})
	} else if snapshot.png != nil {
		img, err := validatePNG(snapshot.png)
		if err != nil {
			return 0, false, err
		}
		format := pngFormat()
		if format == 0 {
			return 0, false, errors.New("无法注册 PNG 剪贴板格式")
		}
		formats = append(formats, struct {
			format uintptr
			data   []byte
		}{format, snapshot.png}, struct {
			format uintptr
			data   []byte
		}{8, imageToDIB(img)})
	} else {
		if len(snapshot.text) > maxText {
			return 0, false, errors.New("文本不能超过 2 MB")
		}
		if snapshot.text != "" {
			units := append(utf16.Encode([]rune(snapshot.text)), 0)
			data := make([]byte, len(units)*2)
			for i, unit := range units {
				binary.LittleEndian.PutUint16(data[i*2:], unit)
			}
			formats = append(formats, struct {
				format uintptr
				data   []byte
			}{13, data})
		}
	}
	superseded := false
	err := c.opened(func() error {
		if expected != nil && *expected != c.revision() {
			superseded = true
			return nil
		}
		result, _, err := emptyClipboard.Call()
		if result == 0 {
			return fmtNative("无法清空剪贴板", err)
		}
		for _, value := range formats {
			if err := putClipboard(value.format, value.data); err != nil {
				return err
			}
		}
		return nil
	})
	return c.revision(), superseded, err
}

func imageToDIB(img image.Image) []byte {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	data := make([]byte, 40+width*height*4)
	binary.LittleEndian.PutUint32(data, 40)
	binary.LittleEndian.PutUint32(data[4:], uint32(width))
	binary.LittleEndian.PutUint32(data[8:], uint32(height))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			offset := 40 + ((height-1-y)*width+x)*4
			data[offset], data[offset+1], data[offset+2], data[offset+3] = pixel.B, pixel.G, pixel.R, 255
		}
	}
	return data
}

// The OS clipboard's DIB is untrusted. Validate every offset and dimension
// before allocating or addressing a pixel. BI_RGB 24/32 and standard BITFIELDS
// cover the WinForms SetImage path as well as common screenshot applications.
func dibToPNG(data []byte) ([]byte, error) {
	invalid := errors.New("剪贴板位图格式无效")
	if len(data) < 40 {
		return nil, invalid
	}
	header := int(binary.LittleEndian.Uint32(data))
	if header < 40 || header > len(data) {
		return nil, invalid
	}
	width, height := int(int32(binary.LittleEndian.Uint32(data[4:]))), int(int32(binary.LittleEndian.Uint32(data[8:])))
	if width <= 0 || height == 0 || height == -2147483648 {
		return nil, invalid
	}
	topDown := height < 0
	if topDown {
		height = -height
	}
	if int64(width)*int64(height) > maxPixels {
		return nil, errors.New("图片不能超过 4000 万像素")
	}
	bits := int(binary.LittleEndian.Uint16(data[14:]))
	compression := binary.LittleEndian.Uint32(data[16:])
	if binary.LittleEndian.Uint16(data[12:]) != 1 || (bits != 24 && bits != 32) || (compression != 0 && compression != 3) {
		return nil, invalid
	}
	offset := header
	if compression == 3 {
		masks := 40
		if header == 40 {
			offset += 12
		}
		if len(data) < masks+12 || binary.LittleEndian.Uint32(data[masks:]) != 0xff0000 || binary.LittleEndian.Uint32(data[masks+4:]) != 0xff00 || binary.LittleEndian.Uint32(data[masks+8:]) != 0xff {
			return nil, invalid
		}
	}
	colors := int(binary.LittleEndian.Uint32(data[32:]))
	if colors > 256 {
		return nil, invalid
	}
	offset += colors * 4
	stride := (width*bits + 31) / 32 * 4
	if offset > len(data) || int64(stride)*int64(height) > int64(len(data)-offset) {
		return nil, invalid
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sourceY := height - 1 - y
		if topDown {
			sourceY = y
		}
		for x := 0; x < width; x++ {
			position := offset + sourceY*stride + x*(bits/8)
			img.SetNRGBA(x, y, color.NRGBA{R: data[position+2], G: data[position+1], B: data[position], A: 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		return nil, err
	}
	if output.Len() > maxPNG {
		return nil, errors.New("图片不能超过 20 MB")
	}
	return output.Bytes(), nil
}
