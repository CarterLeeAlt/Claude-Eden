package desktop

import "math"

type Rectangle struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type WindowPlacement struct {
	Rectangle
	DPI       int  `json:"dpi"`
	Maximized bool `json:"maximized"`
}

func windowBounds(work Rectangle, dpi int, saved *WindowPlacement) (Rectangle, int, int) {
	minimumWidth := min(work.Width, int(math.Round(820*float64(dpi)/96)))
	minimumHeight := min(work.Height, int(math.Round(580*float64(dpi)/96)))
	var width, height, x, y int
	if saved != nil && saved.Width > 0 && saved.Height > 0 && saved.DPI > 0 {
		width = int(math.Round(float64(saved.Width*dpi) / float64(saved.DPI)))
		height = int(math.Round(float64(saved.Height*dpi) / float64(saved.DPI)))
		x, y = saved.X, saved.Y
	} else {
		scale := min(float64(work.Width)*.85/1280, float64(work.Height)*.85/850)
		width, height = int(math.Round(1280*scale)), int(math.Round(850*scale))
		x, y = work.X+(work.Width-width)/2, work.Y+(work.Height-height)/2
	}
	width, height = max(minimumWidth, min(width, work.Width)), max(minimumHeight, min(height, work.Height))
	return Rectangle{max(work.X, min(x, work.X+work.Width-width)), max(work.Y, min(y, work.Y+work.Height-height)), width, height}, minimumWidth, minimumHeight
}
