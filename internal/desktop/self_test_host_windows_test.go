package desktop

import "testing"

func TestNativeTestHostNeedsExplicitAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, mode, manufacturer, product string
		want                              bool
	}{
		{"unapproved-physical", "", "MECHREVO", "wujie14", false},
		{"unapproved-vm", "", "QEMU", "Virtual Machine", false},
		{"vm-flag-on-physical", "vm", "MECHREVO", "wujie14", false},
		{"approved-vm", "vm", "QEMU", "Virtual Machine", true},
		{"explicit-physical", "approved-physical", "MECHREVO", "wujie14", true},
		{"unknown-mode", "physical", "QEMU", "Virtual Machine", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeTestHostAllowed(tc.mode, tc.manufacturer, tc.product); got != tc.want {
				t.Fatalf("authorization=%t; want %t", got, tc.want)
			}
		})
	}
}
