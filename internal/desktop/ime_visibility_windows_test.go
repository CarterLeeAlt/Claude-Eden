package desktop

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestCandidatePixelDifferenceRejectsBlankAndExcludedCarrierChanges(t *testing.T) {
	before, after := image.NewNRGBA(image.Rect(0, 0, 600, 200)), image.NewNRGBA(image.Rect(0, 0, 600, 200))
	region, carrier := before.Bounds(), image.Rect(50, 20, 60, 60)
	if rect, count := candidatePixelDifference(before, after, region, carrier); count != 0 || !rect.Empty() {
		t.Fatal("blank host was accepted")
	}
	draw.Draw(after, carrier, &image.Uniform{color.White}, image.Point{}, draw.Src)
	if rect, count := candidatePixelDifference(before, after, region, carrier); count != 0 || !rect.Empty() {
		t.Fatal("carrier change was accepted as a candidate")
	}
	band := image.Rect(80, 80, 500, 124)
	draw.Draw(after, band, &image.Uniform{color.White}, image.Point{}, draw.Src)
	if rect, count := candidatePixelDifference(before, after, region, carrier); rect != band || count != band.Dx()*band.Dy() {
		t.Fatal("rendered row was not measured exactly", rect, count)
	}
	if rect, count := candidatePixelDifference(before, after, image.Rect(0, 0, 20, 20), carrier); count != 0 || !rect.Empty() {
		t.Fatal("pixels outside the candidate area were included")
	}
}
