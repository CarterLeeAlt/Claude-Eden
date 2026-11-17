package desktop

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (s *Shell) message(message, title string, confirmation bool) (bool, error) {
	return application.InvokeSyncWithResultAndError(func() (bool, error) {
		return showNativeDialog(uintptr(s.window.NativeWindow()), nativeDialogOptions{title: title, message: message, confirmation: confirmation, fonts: s.fonts})
	})
}

func (s *Shell) configureWebViewPolicy(options *application.WebviewWindowOptions, policy WebViewPolicy) error {
	deny := application.CoreWebView2PermissionStateDeny
	options.Windows.GlobalPermission = &deny
	options.Windows.WebViewPolicy = &application.WindowsWebViewPolicy{
		DocumentStartScript: smokeDiagnosticsScript + RuntimeTransportClient,
		AllowNavigation:     func(value application.WebViewNavigation) bool { return policy.AllowNavigation(value.URL) },
		BlockNewWindow:      func(string) bool { return true },
		CancelDownload:      func(string) bool { return true },
		Initialized: func(recovering bool) error {
			s.webFailed.Store(false)
			// This callback has a real HWND. Common WindowShow can be emitted
			// before Wails installs its event mapping during application startup.
			// Resize/focus operations must run after this WebView2 COM callback
			// returns; dispatching synchronously here can re-enter the controller.
			go s.nativeStart.Do(s.startNativeWindow)
			return nil
		},
		InitializationFailed: func(err error) {
			s.webFailed.Store(true)
			s.startupError("无法初始化工作台显示组件：" + err.Error() + "。后台任务保留，可重试连接。")
			go s.nativeStart.Do(s.startNativeWindow)
		},
		NavigationCompleted: func(value application.WebViewNavigationResult) {
			policy.NavigationCompleted(value.Success, fmt.Sprintf("WebView2 错误 %d", value.WebErrorStatus))
		},
		ProcessFailed: func(value application.WebViewProcessFailure) {
			s.webFailed.Store(true)
			policy.ProcessFailed(fmt.Sprintf("类型 %d", value.Kind))
		},
		ScriptDialog: func(value application.WebViewScriptDialog) (application.WebViewScriptDialogResult, error) {
			if !IsWorkbench(value.URL, int(s.expectedPort.Load())) {
				return application.WebViewScriptDialogResult{}, nil
			}
			accepted, err := s.message(value.Message, "Claude Eden", value.Kind == 1 || value.Kind == 3)
			return application.WebViewScriptDialogResult{Accept: accepted}, err
		},
	}
	return nil
}
