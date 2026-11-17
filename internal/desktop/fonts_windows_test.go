package desktop

import "testing"

// Missing-resource paths must return before touching GDI, any HWND, or the
// clipboard. Valid font registration belongs exclusively to VM native tests.
func TestNativeFontsRejectMissingResourcesBeforeGDI(t *testing.T) {
	for _, resources := range []struct{ regular, bold []byte }{
		{}, {regular: []byte{1}}, {bold: []byte{1}},
	} {
		if fonts, err := newNativeFonts(resources.regular, resources.bold); err == nil || fonts != nil {
			t.Fatal("missing embedded font silently fell back to a system font")
		}
		if err := ShowStartupError("fixture", Options{FontRegular: resources.regular, FontBold: resources.bold}); err == nil {
			t.Fatal("startup failure dialog accepted missing embedded fonts")
		}
	}
	if err := verifyNativeFont(0); err == nil {
		t.Fatal("missing GDI font handle accepted")
	}
}

func TestNativeControlsRejectMissingFontsBeforeWindowCreation(t *testing.T) {
	if _, err := showNativeDialog(0, nativeDialogOptions{title: "fixture", message: "fixture"}); err == nil {
		t.Fatal("native dialog accepted missing embedded fonts")
	}
	if _, err := newNativeStatus(0, nil); err == nil {
		t.Fatal("startup status accepted missing embedded fonts")
	}
	for _, fonts := range []*nativeFonts{nil, {}, {family: "Eden UI"}} {
		if err := verifySelfTestControlFonts(0, fonts); err == nil {
			t.Fatal("native self-test accepted an unregistered font")
		}
	}
}
