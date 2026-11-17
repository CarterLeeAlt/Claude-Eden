//go:build windows

package webview2

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Pure native-ABI fixtures: no WebView, clipboard, visible HWND, input or disk
// mutations. CoTaskMemAlloc is used only to match the API's string ownership.
func TestPolicyStringOutParameters(t *testing.T) {
	const value = "http://localhost:4123/中文"
	text, _ := windows.UTF16FromString(value)
	getter := NewComProc(func(this, out uintptr) uintptr {
		if out == 0 {
			return 0x80070057
		}
		memory, _, _ := windows.NewLazySystemDLL("ole32.dll").NewProc("CoTaskMemAlloc").Call(uintptr(len(text) * 2))
		copy(unsafe.Slice((*uint16)(unsafe.Pointer(memory)), len(text)), text)
		*(*uintptr)(unsafe.Pointer(out)) = memory
		return 0
	})
	for _, test := range []struct {
		name string
		get  func() (string, error)
	}{
		{"navigation", (&ICoreWebView2NavigationStartingEventArgs{Vtbl: &ICoreWebView2NavigationStartingEventArgsVtbl{GetUri: getter}}).GetUri},
		{"new window", (&ICoreWebView2NewWindowRequestedEventArgs{Vtbl: &ICoreWebView2NewWindowRequestedEventArgsVtbl{GetUri: getter}}).GetUri},
		{"download", (&ICoreWebView2DownloadOperation{Vtbl: &ICoreWebView2DownloadOperationVtbl{GetUri: getter}}).GetUri},
		{"dialog URI", (&ICoreWebView2ScriptDialogOpeningEventArgs{Vtbl: &ICoreWebView2ScriptDialogOpeningEventArgsVtbl{GetUri: getter}}).GetUri},
		{"dialog message", (&ICoreWebView2ScriptDialogOpeningEventArgs{Vtbl: &ICoreWebView2ScriptDialogOpeningEventArgsVtbl{GetMessage: getter}}).GetMessage},
		{"dialog default", (&ICoreWebView2ScriptDialogOpeningEventArgs{Vtbl: &ICoreWebView2ScriptDialogOpeningEventArgsVtbl{GetDefaultText: getter}}).GetDefaultText},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := test.get()
			if err != nil || actual != value {
				t.Fatalf("out pointer incorrect: %q %v", actual, err)
			}
		})
	}
	runtime.KeepAlive(text)
}

func TestPolicyEventRemovalTokenIsByValue(t *testing.T) {
	token := EventRegistrationToken{value: 0x1122334455667788}
	var actual uintptr
	remove := NewComProc(func(this, value uintptr) uintptr { actual = value; return 0 })
	view := &ICoreWebView2{Vtbl: &ICoreWebView2Vtbl{RemoveNavigationStarting: remove, RemoveNewWindowRequested: remove, RemoveScriptDialogOpening: remove}}
	view4 := &ICoreWebView2_4{Vtbl: &ICoreWebView2_4Vtbl{RemoveDownloadStarting: remove}}
	for _, call := range []func(EventRegistrationToken) error{view.RemoveNavigationStarting, view.RemoveNewWindowRequested, view.RemoveScriptDialogOpening, view4.RemoveDownloadStarting} {
		actual = 0
		if err := call(token); err != nil || actual != uintptr(token.value) {
			t.Fatalf("event token was not passed by value: %x %v", actual, err)
		}
	}
}
