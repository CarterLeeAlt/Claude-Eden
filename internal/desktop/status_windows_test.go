package desktop

import "testing"

func TestStatusControlBoundsCenterVisibleContent(t *testing.T) {
	for _, dpi := range []int32{96, 120, 144, 168, 192} {
		unit := func(v int32) int32 { return v * dpi / 96 }
		for _, size := range [][2]int32{{820, 580}, {1100, 730}, {1500, 1000}} {
			width, height := unit(size[0]), unit(size[1])
			for _, lines := range []int32{1, 4} {
				for _, state := range [][2]bool{{false, false}, {true, false}, {false, true}} {
					text, retry, existing, complete := statusControlBounds(width, height, dpi, unit(24)*lines, state[0], state[1])
					if delta := text.Left + text.Right - width; delta < -1 || delta > 1 {
						t.Fatalf("text off horizontal center at dpi=%d size=%v: %+v", dpi, size, text)
					}
					bottom := text.Bottom
					if state[0] {
						bottom = retry.Bottom
						if delta := retry.Left + retry.Right - width; delta < -1 || delta > 1 {
							t.Fatal("retry off center", retry)
						}
					}
					if state[1] {
						bottom = complete.Bottom
						if delta := existing.Left + complete.Right - width; delta < -1 || delta > 1 {
							t.Fatal("upgrade actions off center", existing, complete)
						}
						if complete.Left-existing.Right != unit(12) {
							t.Fatal("upgrade action spacing changed")
						}
					}
					if delta := text.Top + bottom - height; delta < -1 || delta > 1 {
						t.Fatalf("visible content off vertical center: text=%+v bottom=%d height=%d", text, bottom, height)
					}
				}
			}
		}
	}
}
