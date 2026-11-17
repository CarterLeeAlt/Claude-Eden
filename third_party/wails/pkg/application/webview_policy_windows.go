//go:build windows && !server

package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unsafe"

	"github.com/wailsapp/wails/v3/internal/webview2/pkg/combridge"
	"github.com/wailsapp/wails/v3/internal/webview2/pkg/webview2"
)

type policyDocumentScriptRegistrar interface {
	CallDevTools(string, string, func(string, error)) error
}

func registerPolicyDocumentStartScript(chromium policyDocumentScriptRegistrar, script string, done func(error)) {
	if script == "" {
		done(nil)
		return
	}
	params, err := json.Marshal(map[string]string{"source": script})
	if err != nil {
		done(err)
		return
	}
	if err := chromium.CallDevTools("Page.addScriptToEvaluateOnNewDocument", string(params), func(result string, err error) {
		if err == nil {
			var value struct{ Identifier string }
			err = json.Unmarshal([]byte(result), &value)
			if err == nil && value.Identifier == "" {
				err = errors.New("webview2: document-start script registration returned no identifier")
			}
		}
		done(err)
	}); err != nil {
		done(err)
	}
}

func (w *windowsWebviewWindow) startWebViewPolicy(recovering bool) {
	policy := w.parent.options.Windows.WebViewPolicy
	var script string
	if policy != nil {
		script = policy.DocumentStartScript
	}
	hooks := w.policyHooks
	registerPolicyDocumentStartScript(w.chromium, script, func(err error) {
		if w.parent.isDestroyed() || hooks != nil && hooks.closed {
			return
		}
		if err == nil {
			w.documentStartReady = true
		}
		if err == nil && policy != nil && policy.Initialized != nil {
			err = policy.Initialized(recovering)
		}
		if err != nil {
			w.documentStartReady = false
			w.setupChromiumConfigError(recovering, err)
			hooks.close()
			w.policyHooks = nil
			w.chromium.Close()
			return
		}
		w.startRequestCancellation(w.navigateInitialPage)
	})
}

type policyNavigationHandler interface {
	Invoke(*webview2.ICoreWebView2, *webview2.ICoreWebView2NavigationStartingEventArgs) uintptr
}
type policyNewWindowHandler interface {
	Invoke(*webview2.ICoreWebView2, *webview2.ICoreWebView2NewWindowRequestedEventArgs) uintptr
}
type policyDownloadHandler interface {
	Invoke(*webview2.ICoreWebView2, *webview2.ICoreWebView2DownloadStartingEventArgs) uintptr
}
type policyDialogHandler interface {
	Invoke(*webview2.ICoreWebView2, *webview2.ICoreWebView2ScriptDialogOpeningEventArgs) uintptr
}

type policyNavigationFunc func(*webview2.ICoreWebView2NavigationStartingEventArgs) uintptr

func (f policyNavigationFunc) Invoke(_ *webview2.ICoreWebView2, args *webview2.ICoreWebView2NavigationStartingEventArgs) uintptr {
	return f(args)
}

type policyNewWindowFunc func(*webview2.ICoreWebView2NewWindowRequestedEventArgs) uintptr

func (f policyNewWindowFunc) Invoke(_ *webview2.ICoreWebView2, args *webview2.ICoreWebView2NewWindowRequestedEventArgs) uintptr {
	return f(args)
}

type policyDownloadFunc func(*webview2.ICoreWebView2DownloadStartingEventArgs) uintptr

func (f policyDownloadFunc) Invoke(_ *webview2.ICoreWebView2, args *webview2.ICoreWebView2DownloadStartingEventArgs) uintptr {
	return f(args)
}

type policyDialogFunc func(*webview2.ICoreWebView2ScriptDialogOpeningEventArgs) uintptr

func (f policyDialogFunc) Invoke(_ *webview2.ICoreWebView2, args *webview2.ICoreWebView2ScriptDialogOpeningEventArgs) uintptr {
	return f(args)
}

func init() {
	combridge.RegisterVTable[combridge.IUnknown, policyNavigationHandler]("{9adbe429-f36d-432b-9ddc-f8881fbd76e3}", func(this uintptr, sender *webview2.ICoreWebView2, args *webview2.ICoreWebView2NavigationStartingEventArgs) uintptr {
		return combridge.Resolve[policyNavigationHandler](this).Invoke(sender, args)
	})
	combridge.RegisterVTable[combridge.IUnknown, policyNewWindowHandler]("{d4c185fe-c81c-4989-97af-2d3fa7ab5651}", func(this uintptr, sender *webview2.ICoreWebView2, args *webview2.ICoreWebView2NewWindowRequestedEventArgs) uintptr {
		return combridge.Resolve[policyNewWindowHandler](this).Invoke(sender, args)
	})
	combridge.RegisterVTable[combridge.IUnknown, policyDownloadHandler]("{efedc989-c396-41ca-83f7-07f845a55724}", func(this uintptr, sender *webview2.ICoreWebView2, args *webview2.ICoreWebView2DownloadStartingEventArgs) uintptr {
		return combridge.Resolve[policyDownloadHandler](this).Invoke(sender, args)
	})
	combridge.RegisterVTable[combridge.IUnknown, policyDialogHandler]("{ef381bf9-afa8-4e37-91c4-8ac48524bdfb}", func(this uintptr, sender *webview2.ICoreWebView2, args *webview2.ICoreWebView2ScriptDialogOpeningEventArgs) uintptr {
		return combridge.Resolve[policyDialogHandler](this).Invoke(sender, args)
	})
}

type windowsWebViewPolicyHooks struct {
	closed    bool
	removers  []func() error
	callbacks []io.Closer
	pending   map[uintptr]func()
}

func (h *windowsWebViewPolicyHooks) close() {
	if h == nil || h.closed {
		return
	}
	h.closed = true
	for key, complete := range h.pending {
		complete()
		delete(h.pending, key)
	}
	for index := len(h.removers) - 1; index >= 0; index-- {
		_ = h.removers[index]()
	}
	for _, callback := range h.callbacks {
		_ = callback.Close()
	}
	h.removers = nil
	h.callbacks = nil
}

func (w *windowsWebviewWindow) installWebViewPolicy() (err error) {
	policy := w.parent.options.Windows.WebViewPolicy
	if policy == nil {
		return nil
	}
	controller := w.chromium.GetController()
	if controller == nil {
		return errors.New("webview2: policy requires a controller")
	}
	core, err := controller.GetCoreWebView2()
	if err != nil {
		return err
	}
	defer core.Release()
	// Both internal packages model the same documented WebView2 ABI; the cast is
	// confined to the framework. Application callbacks receive only Go values.
	view := (*webview2.ICoreWebView2)(unsafe.Pointer(core))
	hooks := &windowsWebViewPolicyHooks{pending: make(map[uintptr]func())}
	defer func() {
		if err != nil {
			hooks.close()
		} else {
			w.policyHooks = hooks
		}
	}()
	if policy.AllowNavigation != nil {
		callback := combridge.New[policyNavigationHandler](policyNavigationFunc(func(args *webview2.ICoreWebView2NavigationStartingEventArgs) uintptr {
			if hooks.closed {
				return 0
			}
			defer handlePanic()
			// Set cancellation first: unreadable metadata, a panic or a rejected
			// redirect cannot fall through to a permissive browser default.
			if err := args.PutCancel(true); err != nil {
				globalApplication.handleError(err)
				return 0
			}
			uri, err := args.GetUri()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			user, err := args.GetIsUserInitiated()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			redirected, err := args.GetIsRedirected()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			if policy.AllowNavigation(WebViewNavigation{uri, user, redirected}) {
				if err := args.PutCancel(false); err != nil {
					globalApplication.handleError(err)
				}
			}
			return 0
		}))
		hooks.callbacks = append(hooks.callbacks, callback)
		token, e := view.AddNavigationStarting((*webview2.ICoreWebView2NavigationStartingEventHandler)(unsafe.Pointer(callback.Ref())))
		if e != nil {
			return e
		}
		hooks.removers = append(hooks.removers, func() error { return view.RemoveNavigationStarting(token) })
	}
	if policy.BlockNewWindow != nil {
		callback := combridge.New[policyNewWindowHandler](policyNewWindowFunc(func(args *webview2.ICoreWebView2NewWindowRequestedEventArgs) uintptr {
			if hooks.closed {
				return 0
			}
			defer handlePanic()
			if err := args.PutHandled(true); err != nil {
				globalApplication.handleError(err)
				return 0
			}
			uri, err := args.GetUri()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			if !policy.BlockNewWindow(uri) {
				if err := args.PutHandled(false); err != nil {
					globalApplication.handleError(err)
				}
			}
			return 0
		}))
		hooks.callbacks = append(hooks.callbacks, callback)
		token, e := view.AddNewWindowRequested((*webview2.ICoreWebView2NewWindowRequestedEventHandler)(unsafe.Pointer(callback.Ref())))
		if e != nil {
			return e
		}
		hooks.removers = append(hooks.removers, func() error { return view.RemoveNewWindowRequested(token) })
	}
	if policy.CancelDownload != nil {
		view4 := view.GetICoreWebView2_4()
		if view4 == nil {
			return errors.New("webview2: DownloadStarting policy is unsupported by the installed runtime")
		}
		hooks.removers = append(hooks.removers, func() error { view4.Vtbl.Release.Call(uintptr(unsafe.Pointer(view4))); return nil })
		callback := combridge.New[policyDownloadHandler](policyDownloadFunc(func(args *webview2.ICoreWebView2DownloadStartingEventArgs) uintptr {
			if hooks.closed {
				return 0
			}
			defer handlePanic()
			if err := args.PutCancel(true); err != nil {
				globalApplication.handleError(err)
				return 0
			}
			operation, err := args.GetDownloadOperation()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			defer operation.Vtbl.Release.Call(uintptr(unsafe.Pointer(operation)))
			uri, err := operation.GetUri()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			if !policy.CancelDownload(uri) {
				if err := args.PutCancel(false); err != nil {
					globalApplication.handleError(err)
				}
			}
			return 0
		}))
		hooks.callbacks = append(hooks.callbacks, callback)
		token, e := view4.AddDownloadStarting((*webview2.ICoreWebView2DownloadStartingEventHandler)(unsafe.Pointer(callback.Ref())))
		if e != nil {
			return e
		}
		hooks.removers = append(hooks.removers, func() error { return view4.RemoveDownloadStarting(token) })
	}
	if policy.ScriptDialog != nil {
		settings, e := w.chromium.GetSettings()
		if e != nil {
			return e
		}
		e = settings.PutAreDefaultScriptDialogsEnabled(false)
		settings.Release()
		if e != nil {
			return e
		}
		callback := combridge.New[policyDialogHandler](policyDialogFunc(func(args *webview2.ICoreWebView2ScriptDialogOpeningEventArgs) uintptr {
			if hooks.closed {
				return 0
			}
			defer handlePanic()
			uri, err := args.GetUri()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			kind, err := args.GetKind()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			message, err := args.GetMessage()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			text, err := args.GetDefaultText()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			deferral, err := args.GetDeferral()
			if err != nil {
				globalApplication.handleError(err)
				return 0
			}
			args.AddRef()
			key := uintptr(unsafe.Pointer(args))
			hooks.pending[key] = func() {
				_ = deferral.Complete()
				args.Vtbl.Release.Call(uintptr(unsafe.Pointer(args)))
				deferral.Vtbl.Release.Call(uintptr(unsafe.Pointer(deferral)))
			}
			go func() {
				var result WebViewScriptDialogResult
				var dialogError error
				defer func() {
					if recovered := recover(); recovered != nil {
						dialogError = fmt.Errorf("webview2 script dialog panic: %v", recovered)
					}
					InvokeAsync(func() {
						complete, live := hooks.pending[key]
						if !live {
							return
						}
						delete(hooks.pending, key)
						defer complete()
						if dialogError != nil {
							globalApplication.handleError(dialogError)
							return
						}
						if hooks.closed {
							return
						}
						if result.Accept {
							if kind == 2 {
								if err := args.PutResultText(result.Text); err != nil {
									globalApplication.handleError(err)
									return
								}
							}
							if err := args.Accept(); err != nil {
								globalApplication.handleError(err)
							}
						}
					})
				}()
				result, dialogError = policy.ScriptDialog(WebViewScriptDialog{uri, uint32(kind), message, text})
			}()
			return 0
		}))
		hooks.callbacks = append(hooks.callbacks, callback)
		token, e := view.AddScriptDialogOpening((*webview2.ICoreWebView2ScriptDialogOpeningEventHandler)(unsafe.Pointer(callback.Ref())))
		if e != nil {
			return e
		}
		hooks.removers = append(hooks.removers, func() error { return view.RemoveScriptDialogOpening(token) })
	}
	return nil
}

// RecoverWebView explicitly retries a failed Windows controller. It preserves
// the native HWND and reapplies all options/policies before loading content.
func (window *WebviewWindow) RecoverWebView() error {
	return InvokeSyncWithError(func() error {
		w, ok := window.impl.(*windowsWebviewWindow)
		if !ok || w.hwnd == 0 || window.isDestroyed() {
			return errors.New("webview2: window is unavailable")
		}
		w.resetWebviewRecoveryBudget()
		w.rebuildWebView()
		if w.chromium.GetController() == nil {
			return errors.New("webview2: controller recovery did not complete")
		}
		return nil
	})
}
