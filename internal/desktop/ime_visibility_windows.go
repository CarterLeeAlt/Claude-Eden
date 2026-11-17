package desktop

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"unsafe"
)

// Composited Windows 10 IME candidates may be absent from EnumWindows and UIA.
// Read the actual owned-window pixels, excluding the transient EDIT, and require
// a full changed candidate row rather than a caret or one-pixel text change.
func renderedIMECandidate(input *nativeInput, beforePath, afterPath string) (bool, string, error) {
	read := func(path string) (image.Image, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return png.Decode(file)
	}
	before, err := read(beforePath)
	if err != nil {
		return false, "", err
	}
	after, err := read(afterPath)
	if err != nil {
		return false, "", err
	}
	if before.Bounds() != after.Bounds() {
		return false, "", errors.New("输入法候选截图期间自有窗口尺寸发生变化")
	}
	var parent, edit winRect
	user32.NewProc("GetWindowRect").Call(input.parent, uintptr(unsafe.Pointer(&parent)))
	user32.NewProc("GetWindowRect").Call(input.hwnd, uintptr(unsafe.Pointer(&edit)))
	left, top, right, bottom := int(edit.Left-parent.Left), int(edit.Top-parent.Top), int(edit.Right-parent.Left), int(edit.Bottom-parent.Top)
	region := image.Rect(left-40, top-80, after.Bounds().Max.X, bottom+140).Intersect(after.Bounds())
	excluded := image.Rect(left-4, top-4, right+4, bottom+4)
	changed, pixels := candidatePixelDifference(before, after, region, excluded)
	visible := changed.Dx() >= 160 && changed.Dy() >= 20 && changed.Dy() <= 100 && pixels >= 500
	return visible, fmt.Sprintf("候选区真实屏幕像素变化：范围=%v，像素=%d，排除 EDIT=%v", changed, pixels, excluded), nil
}

func candidatePixelDifference(before, after image.Image, region, excluded image.Rectangle) (image.Rectangle, int) {
	region = region.Intersect(before.Bounds()).Intersect(after.Bounds())
	var changed image.Rectangle
	pixels := 0
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			point := image.Pt(x, y)
			if point.In(excluded) {
				continue
			}
			a, b := color.NRGBAModel.Convert(before.At(x, y)).(color.NRGBA), color.NRGBAModel.Convert(after.At(x, y)).(color.NRGBA)
			difference := max(int(a.R), int(b.R)) - min(int(a.R), int(b.R)) + max(int(a.G), int(b.G)) - min(int(a.G), int(b.G)) + max(int(a.B), int(b.B)) - min(int(a.B), int(b.B))
			if difference < 24 {
				continue
			}
			pixels++
			changed = changed.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return changed, pixels
}

func verifyIMECandidateClosed(input *nativeInput, output, stage string, report *NativeSelfTestReport) error {
	name := "Native_IME_After_" + stage + ".png"
	if err := captureVisibleNativeWindow(input.parent, filepath.Join(output, name)); err != nil {
		return err
	}
	report.Screenshots = append(report.Screenshots, name)
	visible, note, err := renderedIMECandidate(input, filepath.Join(output, "Native_IME_Before.png"), filepath.Join(output, name))
	if err != nil {
		return err
	}
	if visible {
		return fmt.Errorf("%s 后候选条仍然显示：%s", stage, note)
	}
	report.Checks = append(report.Checks, "实际屏幕确认候选条在 "+stage+" 后消失，未用空宿主截图代替可见性")
	return nil
}
