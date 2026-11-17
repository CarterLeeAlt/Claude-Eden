package desktop

import (
	"bytes"
	"errors"
	"fmt"
	"syscall"
	"time"
)

type selfTestClipboardItem struct {
	format uintptr
	data   []byte
}

func selfTestClipboardMemoryFormat(format uintptr) bool {
	switch format {
	case 1, 7, 8, 13, 15, 16, 17:
		return true
	}
	// RegisterClipboardFormatW documents both this range and the requirement
	// that registered formats contain HGLOBAL data. Private formats, bitmaps,
	// palettes and metafiles do not share that contract and remain rejected.
	// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-registerclipboardformatw
	return format >= 0xc000 && format <= 0xffff
}

// The caller holds OpenClipboard on one OS thread. No clipboard bytes are
// logged or written to disk; any unreadable format aborts before mutation.
func captureSelfTestClipboard() ([]selfTestClipboardItem, error) {
	var saved []selfTestClipboardItem
	total := 0
	previous := uintptr(0)
	for {
		kernel32.NewProc("SetLastError").Call(0)
		format, _, nativeErr := user32.NewProc("EnumClipboardFormats").Call(previous)
		if format == 0 {
			if nativeErr != syscall.Errno(0) {
				return nil, fmtNative("无法完整保存自检前剪贴板格式", nativeErr)
			}
			return saved, nil
		}
		previous = format
		if !selfTestClipboardMemoryFormat(format) {
			return nil, fmt.Errorf("剪贴板含不能保证完整还原的格式 %d，未改写原内容", format)
		}
		if len(saved) >= 1024 {
			return nil, errors.New("剪贴板格式过多，未改写原内容")
		}
		data, err := clipboardBytes(format, maxPixels*4+4096-total)
		if err != nil {
			return nil, fmt.Errorf("保存剪贴板格式 %d：%w", format, err)
		}
		total += len(data)
		saved = append(saved, selfTestClipboardItem{format, data})
	}
}

func verifySelfTestClipboard(saved, restored []selfTestClipboardItem) error {
	expected := make(map[uintptr][]byte, len(saved))
	for _, item := range saved {
		expected[item.format] = item.data
	}
	_, unicodeText := expected[13]
	for _, item := range restored {
		data, ok := expected[item.format]
		if !ok {
			// Closing a Unicode clipboard can add these two Windows-synthesized
			// representations. They are not discarded original user formats.
			if unicodeText && (item.format == 1 || item.format == 7) {
				continue
			}
			return fmt.Errorf("恢复剪贴板出现非预期格式 %d", item.format)
		}
		if !bytes.Equal(data, item.data) {
			return fmt.Errorf("恢复剪贴板格式 %d 的完整字节回读不一致", item.format)
		}
		delete(expected, item.format)
	}
	if len(expected) != 0 {
		return errors.New("恢复剪贴板缺少原有格式")
	}
	return nil
}

// Both native and production-UI probes use the same preservation contract.
// Restoring is checked once while open and again after CloseClipboard, when
// Windows may synthesize formats. A successful SetClipboardData alone is not
// evidence that the user's original mixed-format content was restored.
func preserveUIClipboard(clipboard nativeClipboard) (func() error, error) {
	var saved []selfTestClipboardItem
	err := clipboard.opened(func() (err error) {
		saved, err = captureSelfTestClipboard()
		return err
	})
	if err != nil {
		return nil, err
	}
	return func() error {
		// Clipboard listeners can briefly hold the clipboard after each write.
		// Retry only opening, never repeat a partially completed restoration.
		opened := false
		deadline := time.Now().Add(2 * time.Second)
		for {
			err = clipboard.opened(func() error {
				opened = true
				if result, _, nativeErr := emptyClipboard.Call(); result == 0 {
					return fmtNative("无法恢复自检前剪贴板", nativeErr)
				}
				for _, item := range saved {
					if err := putClipboard(item.format, item.data); err != nil {
						return err
					}
				}
				restored, err := captureSelfTestClipboard()
				if err != nil {
					return err
				}
				return verifySelfTestClipboard(saved, restored)
			})
			if opened || time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil {
			return err
		}
		return clipboard.opened(func() error {
			restored, err := captureSelfTestClipboard()
			if err != nil {
				return err
			}
			return verifySelfTestClipboard(saved, restored)
		})
	}, nil
}
