package desktop

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkbenchOrigins(t *testing.T) {
	for _, value := range []string{"http://localhost:4281/#secret", "http://localhost:4281/project?x=1#secret"} {
		entry, err := WorkbenchEntry(value)
		if err != nil || entry.Port() != "4281" {
			t.Fatalf("valid entry rejected: %v", err)
		}
	}
	for _, value := range []string{"https://localhost:4281/#t", "http://127.0.0.1:4281/#t", "http://localhost.evil:4281/#t", "http://user@localhost:4281/#t", "http://localhost:4281/", "http://localhost:0/#t", "http://localhost:65536/#t", "file:///local#t", "http://localhost:4281@evil/#t", "%"} {
		if _, err := WorkbenchEntry(value); err == nil {
			t.Errorf("untrusted entry accepted: %q", value)
		}
	}
	for _, value := range []string{"http://localhost:4282/", "http://localhost:42810/", "http://localhost:4281.evil/", "http://localhost:4281\\@evil/"} {
		if IsWorkbench(value, 4281) {
			t.Errorf("unexpected origin accepted: %q", value)
		}
	}
}

func TestInstanceIdentityIncludesUserAndConfiguration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Profile")
	key := instanceKey("S-1-5-21-1", root)
	if len(key) != 24 {
		t.Fatalf("unexpected key: %q", key)
	}
	if key != instanceKey("S-1-5-21-1", strings.ToUpper(root)+string(filepath.Separator)) {
		t.Fatal("Windows path casing/trailing separators changed identity")
	}
	if key == instanceKey("S-1-5-21-2", root) || key == instanceKey("S-1-5-21-1", root+"-other") {
		t.Fatal("distinct user/profile shared an instance")
	}
}

func TestWindowPlacementAcrossWorkAreasAndDPI(t *testing.T) {
	for _, work := range []Rectangle{{0, 0, 1920, 1080}, {-1920, 40, 1920, 1040}, {40, -1080, 800, 600}, {0, 0, 640, 480}, {0, 0, 3840, 2160}} {
		for _, dpi := range []int{96, 120, 144, 192, 240} {
			for _, saved := range []*WindowPlacement{nil, {Rectangle: Rectangle{5000, 5000, 100, 100}, DPI: 96}, {Rectangle: Rectangle{-5000, -5000, 5000, 5000}, DPI: 192}, {Rectangle: Rectangle{10, 20, 1000, 700}, DPI: 120}, {Rectangle: Rectangle{1, 1, -1, 2}, DPI: 0}} {
				bounds, minW, minH := windowBounds(work, dpi, saved)
				if bounds.Width < minW || bounds.Height < minH || bounds.X < work.X || bounds.Y < work.Y || bounds.X+bounds.Width > work.X+work.Width || bounds.Y+bounds.Height > work.Y+work.Height {
					t.Fatalf("placement escaped work area: %+v %+v dpi=%d", bounds, work, dpi)
				}
			}
		}
	}
}

func TestRemoteInputValidation(t *testing.T) {
	request := RemoteInputRequest{Owner: "f32b2e85-9e10-4653-848d-9cd32d2178e0", Operation: "activate", Height: 24}
	if !validInput(request) {
		t.Fatal("valid activation rejected")
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 32769, -32769} {
		copy := request
		copy.X = value
		if validInput(copy) {
			t.Error("invalid X accepted")
		}
		copy = request
		copy.Y = value
		if validInput(copy) {
			t.Error("invalid Y accepted")
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), 0, -1, 4097} {
		copy := request
		copy.Height = value
		if validInput(copy) {
			t.Error("invalid height accepted")
		}
	}
	request.Owner = "other"
	if validInput(request) {
		t.Fatal("invalid owner accepted")
	}
}
