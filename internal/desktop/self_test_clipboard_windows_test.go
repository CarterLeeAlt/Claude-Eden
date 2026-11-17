package desktop

import "testing"

// Pure checks only: ordinary go test must never touch the host clipboard.
func TestSelfTestClipboardMemoryFormats(t *testing.T) {
	for _, format := range []uintptr{1, 7, 8, 13, 15, 16, 17, 0xc000, 49864, 0xffff} {
		if !selfTestClipboardMemoryFormat(format) {
			t.Errorf("HGLOBAL format %d rejected", format)
		}
	}
	for _, format := range []uintptr{0, 2, 3, 9, 14, 0x80, 0x81, 0x200, 0x300, 0xbfff, 0x10000} {
		if selfTestClipboardMemoryFormat(format) {
			t.Errorf("unsupported handle format %d accepted", format)
		}
	}
}

func TestVerifySelfTestClipboard(t *testing.T) {
	saved := []selfTestClipboardItem{{49864, []byte{0}}, {13, []byte{65, 0, 0, 0}}, {16, []byte{9, 4, 0, 0}}}
	for _, tc := range []struct {
		name      string
		data      []selfTestClipboardItem
		wantError bool
	}{
		{"identical", saved, false},
		{"windows-text-conversions", append(append([]selfTestClipboardItem{}, saved...), selfTestClipboardItem{1, []byte{65, 0}}, selfTestClipboardItem{7, []byte{65, 0}}), false},
		{"missing-owner", saved[1:], true},
		{"missing-locale", saved[:2], true},
		{"changed-owner", []selfTestClipboardItem{{49864, []byte{1}}, saved[1], saved[2]}, true},
		{"truncated-text", []selfTestClipboardItem{saved[0], {13, []byte{65, 0}}, saved[2]}, true},
		{"extra-registered-format", append(append([]selfTestClipboardItem{}, saved...), selfTestClipboardItem{0xc000, []byte{0}}), true},
		{"empty-restoration", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifySelfTestClipboard(saved, tc.data); (err != nil) != tc.wantError {
				t.Fatalf("verification error=%v; wantError=%t", err, tc.wantError)
			}
		})
	}
	if err := verifySelfTestClipboard(nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := verifySelfTestClipboard(nil, []selfTestClipboardItem{{1, []byte{0}}}); err == nil {
		t.Fatal("unexpected text accepted for originally empty clipboard")
	}
}
