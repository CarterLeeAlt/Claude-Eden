//go:build windows

package edge

import (
	"testing"
	"unsafe"
)

// Only COM-shaped memory callbacks run here; no WebView or Windows permission
// UI is created. The enforcing global policy must also cover future enum kinds.
func TestPolicyGlobalDenyCoversUnknownPermissionsAndEnforcementErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		kindHR uintptr
		putHR  uintptr
	}{
		{"unknown kind", 0, 0},
		{"unreadable kind", runtimeFailure, 0},
		{"deny cannot be applied", 0, runtimeFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			var applied CoreWebView2PermissionState
			callbacks := 0
			args := &iCoreWebView2PermissionRequestedEventArgs{vtbl: &iCoreWebView2PermissionRequestedEventArgsVtbl{
				GetPermissionKind: NewComProc(func(_ uintptr, out uintptr) uintptr {
					*(*int32)(unsafe.Pointer(out)) = 0x7fffffff
					return test.kindHR
				}),
				PutState: NewComProc(func(_ uintptr, state uintptr) uintptr {
					applied = CoreWebView2PermissionState(state)
					return test.putHR
				}),
			}}
			deny := CoreWebView2PermissionStateDeny
			chromium := fatalOnErrorCallback(t)
			chromium.globalPermission = &deny
			chromium.PermissionErrorCallback = func(err error) {
				if err == nil {
					t.Fatal("enforcement failure lost the original COM error")
				}
				callbacks++
			}
			chromium.PermissionRequested(nil, args)
			wantCallbacks := 0
			if test.putHR != 0 {
				wantCallbacks = 1
			}
			if applied != deny || callbacks != wantCallbacks {
				t.Fatalf("global deny=%v, enforcement callbacks=%d", applied, callbacks)
			}
		})
	}
}
