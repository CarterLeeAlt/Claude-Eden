//go:build windows && !server

package application

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/wailsapp/wails/v3/internal/webview2/pkg/combridge"
	"github.com/wailsapp/wails/v3/internal/webview2/pkg/webview2"
)

type policyScriptFixture struct {
	t        *testing.T
	callback func(string, error)
	err      error
}

func (f *policyScriptFixture) CallDevTools(method, params string, callback func(string, error)) error {
	f.t.Helper()
	var request struct{ Source string }
	if method != "Page.addScriptToEvaluateOnNewDocument" || json.Unmarshal([]byte(params), &request) != nil || request.Source != "fixture script" {
		f.t.Fatal("document-start registration used the wrong native command or script")
	}
	f.callback = callback
	return f.err
}

func TestWindowsPolicyDocumentScriptWaitsForRegistrationAndPreservesFailures(t *testing.T) {
	for _, test := range []struct {
		name, result          string
		immediate, completion error
		wantError             bool
	}{
		{name: "success", result: `{"identifier":"registered"}`},
		{name: "native registration failure", immediate: errors.New("registration failure"), wantError: true},
		{name: "asynchronous failure", completion: errors.New("async registration failure"), wantError: true},
		{name: "missing identifier", result: `{}`, wantError: true},
		{name: "invalid result", result: `broken`, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &policyScriptFixture{t: t, err: test.immediate}
			calls := 0
			var result error
			registerPolicyDocumentStartScript(fixture, "fixture script", func(err error) { calls++; result = err })
			if test.immediate == nil {
				if calls != 0 {
					t.Fatal("navigation may start before registration completes")
				}
				fixture.callback(test.result, test.completion)
			}
			if calls != 1 || (result != nil) != test.wantError {
				t.Fatalf("callback count=%d, error=%v", calls, result)
			}
			if test.immediate != nil && result != test.immediate || test.completion != nil && result != test.completion {
				t.Fatal("native registration error was replaced")
			}
		})
	}
	called := false
	registerPolicyDocumentStartScript(nil, "", func(err error) { called = err == nil })
	if !called {
		t.Fatal("empty script changed ordinary startup")
	}
}

func TestWindowsPolicyDefersApplicationURLUntilDocumentScriptIsRegistered(t *testing.T) {
	window := &windowsWebviewWindow{parent: &WebviewWindow{options: WebviewWindowOptions{Windows: WindowsWindow{WebViewPolicy: &WindowsWebViewPolicy{DocumentStartScript: "fixture script"}}}}}
	// A nil Chromium is intentional: even a backend that resolves before the
	// browser exists must only update the pending URL, never navigate early.
	for _, url := range []string{"http://localhost:7654/", "http://localhost:7654/current"} {
		window.setURL(url)
		if window.lastNavigatedURL != url || window.documentStartReady {
			t.Fatal("application navigation bypassed pending document script registration")
		}
	}
}

type policyCloseFunc func() error

func (f policyCloseFunc) Close() error { return f() }

func TestWindowsPolicyHookCleanupIsReverseAndIdempotent(t *testing.T) {
	var calls []string
	hooks := &windowsWebViewPolicyHooks{removers: []func() error{func() error { calls = append(calls, "release core"); return nil }, func() error { calls = append(calls, "remove event"); return nil }}, callbacks: []io.Closer{policyCloseFunc(func() error { calls = append(calls, "close callback"); return nil })}, pending: map[uintptr]func(){1: func() { calls = append(calls, "complete dialog") }}}
	hooks.close()
	hooks.close()
	if !reflect.DeepEqual(calls, []string{"complete dialog", "remove event", "release core", "close callback"}) {
		t.Fatalf("cleanup order: %v", calls)
	}
}

func TestWindowsPolicyInitializationFailureCanRemainNative(t *testing.T) {
	previous := globalApplication
	globalApplication = &App{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	t.Cleanup(func() { globalApplication = previous })
	var actual error
	w := &windowsWebviewWindow{parent: &WebviewWindow{options: WebviewWindowOptions{Windows: WindowsWindow{WebViewPolicy: &WindowsWebViewPolicy{InitializationFailed: func(err error) { actual = err }}}}}}
	want := errors.New("policy fixture")
	if w.setupChromiumConfigError(false, want) || actual != want {
		t.Fatal("initial policy failure was not propagated without terminating")
	}
}

func TestWindowsPolicyNavigationCOMBridge(t *testing.T) {
	called := false
	callback := combridge.New[policyNavigationHandler](policyNavigationFunc(func(args *webview2.ICoreWebView2NavigationStartingEventArgs) uintptr { called = true; return 0 }))
	defer callback.Close()
	view := (*webview2.ICoreWebView2NavigationStartingEventHandler)(unsafe.Pointer(callback.Ref()))
	result, _, _ := view.Vtbl.Invoke.Call(callback.Ref(), 0, 0)
	if result != 0 || !called {
		t.Fatal("native COM trampoline did not invoke policy")
	}
}
