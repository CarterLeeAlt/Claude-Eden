package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// This fixture controls every byte of its localhost page. It verifies real
// WebView2 + Wails + DesktopService IPC without starting any controller/VM and
// without substituting its relaxed test page for the production security hook.
func runWebviewSelfTest(output string, report *NativeSelfTestReport, options Options, fonts *nativeFonts) error {
	modulePath := os.Getenv("EDEN_NATIVE_PROBE_MODULE")
	if modulePath == "" {
		return errors.New("需指定 EDEN_NATIVE_PROBE_MODULE，使用实际 npm runtime 与生成的 DesktopService bindings 构建 VM 探针模块")
	}
	module, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	var unexpectedRuntimeRequests atomic.Int32
	var acceptedCalls atomic.Uint64
	var blockedNavigations, blockedPopups, blockedDownloads, scriptDialogs atomic.Uint64
	var scriptDialogFontChecks atomic.Uint64
	var controllerInitializations atomic.Uint64
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wails/runtime" {
			unexpectedRuntimeRequests.Add(1)
		}
		if r.URL.Path == "/native-probe.js" {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(module)
			return
		}
		if r.URL.Path == "/probe-state" {
			started, exited := probeBackendState(output)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"backendStarted": started, "backendExited": exited, "acceptedCalls": acceptedCalls.Load(), "blockedNavigations": blockedNavigations.Load(), "blockedPopups": blockedPopups.Load(), "blockedDownloads": blockedDownloads.Load(), "scriptDialogs": scriptDialogs.Load(), "controllerInitializations": controllerInitializations.Load()})
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>Claude Eden WebView2 自检</title><style>body{font:16px 'Segoe UI',sans-serif;background:#f7f7f7;color:#1a1c1f;padding:36px}pre{white-space:pre-wrap}</style><h1>Claude Eden WebView2 自检</h1><p>受控 localhost 页面；不启动后台、虚拟机或安装器。</p><pre id="result">等待 Wails runtime 与类型化桌面服务…</pre><script type="module" src="/native-probe.js"></script></html>`))
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shell, err := New(Options{BackendExecutable: filepath.Join(output, "unused", "eden.exe"), ConfigPath: filepath.Join(output, "profile", "config.json"), Icon: options.Icon, FontRegular: options.FontRegular, FontBold: options.FontBold})
	if err != nil {
		return err
	}
	defer shell.cancel()
	shell.fonts = fonts
	program, err := os.Executable()
	if err != nil {
		return err
	}
	shell.backend.Executable = program
	shell.backend.Directory = output
	shell.backend.prefix = []string{"--native-test-backend"}
	shell.backend.environment = []string{"EDEN_NATIVE_TEST_OUTPUT=" + output, "EDEN_NATIVE_TEST_ENTRY=http://localhost:" + strconv.Itoa(port) + "/#native-self-test"}
	transport := &nativeTransport{port: func() int { return port }, observe: func(application.RuntimeRequest) { acceptedCalls.Add(1) }}
	type pageResult struct {
		Success bool     `json:"success"`
		Checks  []string `json:"checks"`
		Error   string   `json:"error"`
	}
	var mu sync.Mutex
	var result *pageResult
	var once sync.Once
	var fixtureWindow *application.WebviewWindow
	var app *application.App
	complete := func(value pageResult) {
		once.Do(func() {
			addFailure := func(message string) {
				value.Success = false
				if value.Error != "" {
					value.Error += "; "
				}
				value.Error += message
			}
			pageContext, pageCancel := context.WithTimeout(ctx, 3*time.Second)
			pageError := shell.captureSmokePage(pageContext)
			pageCancel()
			if pageError != nil {
				addFailure(pageError.Error())
			}
			pageFailures := shell.smokeDiagnostics.pageErrors()
			for _, failure := range pageFailures {
				addFailure("探针页面诊断：" + startupPageMessage(failure))
			}
			if pageError == nil && len(pageFailures) == 0 {
				report.Checks = append(report.Checks, "probe 结束前取得实际文档诊断快照，无 error/module-load/unhandledrejection；取消后的迟到回执没有产生未处理异常")
			}
			fileError := runFileDialogSelfTest(shell, output, report)
			if fileError != nil {
				addFailure(fileError.Error())
			}
			mu.Lock()
			result = &value
			mu.Unlock()
			if fixtureWindow != nil && fixtureWindow.NativeWindow() != nil {
				captureError := application.InvokeSyncWithError(func() error {
					return captureNativeWindow(uintptr(fixtureWindow.NativeWindow()), filepath.Join(output, "Native_WebView2.png"))
				})
				if captureError != nil {
					mu.Lock()
					result.Success = false
					if result.Error != "" {
						result.Error += "; "
					}
					result.Error += captureError.Error()
					mu.Unlock()
				}
			}
			go func() { time.Sleep(100 * time.Millisecond); app.Quit() }()
		})
	}
	app = application.New(application.Options{
		Name: "Claude Eden 原生 WebView2 自检", Icon: options.Icon, Services: []application.Service{application.NewService(shell.service)}, Transport: transport,
		Assets:  application.AssetOptions{Handler: http.NotFoundHandler(), DisableLogging: true},
		Windows: application.WindowsOptions{WebviewUserDataPath: filepath.Join(output, "webview2")},
		RawMessageHandler: func(window application.Window, message string, origin *application.OriginInfo) {
			if failures := shell.smokeDiagnostics.receive(message, origin, port); len(failures) != 0 {
				shell.pageStartupError(failures[0])
			}
			if message == "eden-selftest-rebuild-webview" {
				if trustedWorkbenchSource(origin, port) {
					go func() {
						if err := fixtureWindow.RecoverWebView(); err != nil {
							complete(pageResult{Error: err.Error()})
						}
					}()
				}
				return
			}
			if message == "eden-selftest-emit-event" {
				if trustedWorkbenchSource(origin, port) {
					app.Event.Emit("eden-native-input", InputEvent{Owner: "bd799ff2-597d-4e24-a0a4-7db302d6b981", Kind: "compositionupdate", Text: "Go Event.Emit 真实数据", SelectionStart: 2, SelectionEnd: 2})
				}
				return
			}
			if strings.HasPrefix(message, "eden-selftest-result:") {
				if !trustedWorkbenchSource(origin, port) {
					return
				}
				var value pageResult
				if len(message) > 65536 || json.Unmarshal([]byte(strings.TrimPrefix(message, "eden-selftest-result:")), &value) != nil {
					return
				}
				go complete(value)
				return
			}
			transport.handle(window, message, origin)
		},
		ErrorHandler: func(err error) { go complete(pageResult{Error: err.Error()}) },
		PanicHandler: func(details *application.PanicDetails) { go complete(pageResult{Error: details.Error.Error()}) },
		OnShutdown: func() {
			cancel()
			if shell.input != nil {
				application.InvokeSync(shell.input.close)
			}
		},
	})
	shell.app = app
	deny := application.CoreWebView2PermissionStateDeny
	windowOptions := application.WebviewWindowOptions{Name: "native-selftest", Title: "Claude Eden WebView2 自检", URL: "http://localhost:" + strconv.Itoa(port) + "/#native-self-test", Width: 920, Height: 620, BackgroundColour: application.NewRGB(247, 247, 247), DefaultContextMenuDisabled: true, Windows: application.WindowsWindow{Theme: application.Light, DisableMenu: true, GlobalPermission: &deny}}
	windowOptions.Windows.WebViewPolicy = &application.WindowsWebViewPolicy{
		DocumentStartScript: smokeDiagnosticsScript + RuntimeTransportClient,
		Initialized:         func(bool) error { controllerInitializations.Add(1); return nil },
		AllowNavigation: func(value application.WebViewNavigation) bool {
			allowed := IsWorkbench(value.URL, port)
			if !allowed {
				blockedNavigations.Add(1)
			}
			return allowed
		},
		BlockNewWindow:       func(string) bool { blockedPopups.Add(1); return true },
		CancelDownload:       func(string) bool { blockedDownloads.Add(1); return true },
		InitializationFailed: func(err error) { go complete(pageResult{Error: err.Error()}) },
		ScriptDialog: func(value application.WebViewScriptDialog) (application.WebViewScriptDialogResult, error) {
			scriptDialogs.Add(1)
			accepted, err := application.InvokeSyncWithResultAndError(func() (bool, error) {
				var probeError error
				accepted, dialogError := showNativeDialog(uintptr(fixtureWindow.NativeWindow()), nativeDialogOptions{title: "Claude Eden 脚本对话框自检", message: value.Message, confirmation: value.Kind == 1 || value.Kind == 3, fonts: fonts, timer: func(hwnd uintptr) {
					probeError = errors.Join(verifySelfTestControlFonts(hwnd, fonts), captureNativeWindow(hwnd, filepath.Join(output, "Native_WebView_Script_Dialog.png")))
					user32.NewProc("PostMessageW").Call(hwnd, 0x0111, 1, 0)
				}})
				if dialogError == nil && probeError == nil {
					scriptDialogFontChecks.Add(1)
				}
				return accepted, errors.Join(dialogError, probeError)
			})
			return application.WebViewScriptDialogResult{Accept: accepted}, err
		},
	}
	fixtureWindow = app.Window.NewWithOptions(windowOptions)
	shell.window = fixtureWindow
	fixtureWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		err := application.InvokeSyncWithError(func() error {
			if shell.input != nil {
				shell.input.close()
			}
			input, err := newNativeInput(uintptr(fixtureWindow.NativeWindow()), func(value InputEvent) { app.Event.Emit("eden-native-input", value) })
			if err == nil {
				shell.input = input
				shell.clipboard.native.hwnd = uintptr(fixtureWindow.NativeWindow())
			}
			return err
		})
		if err != nil {
			complete(pageResult{Error: err.Error()})
			return
		}
	})
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(90 * time.Second):
			complete(pageResult{Error: "WebView2/类型化绑定自检超过 90 秒"})
		}
	}()
	err = app.Run()
	if err != nil {
		return err
	}
	mu.Lock()
	value := result
	mu.Unlock()
	if value == nil {
		return errors.New("WebView2 自检窗口未返回页面结果")
	}
	report.Checks = append(report.Checks, value.Checks...)
	for _, name := range []string{"Native_WebView2.png", "Native_WebView_Script_Dialog.png"} {
		if _, err := os.Stat(filepath.Join(output, name)); err == nil {
			report.Screenshots = append(report.Screenshots, name)
		}
	}
	if !value.Success {
		return errors.New("WebView2 自检未通过：" + value.Error)
	}
	if unexpectedRuntimeRequests.Load() != 0 {
		return errors.New("Wails 运行时请求错误地到达控制器 HTTP 路由")
	}
	if scriptDialogFontChecks.Load() == 0 || scriptDialogFontChecks.Load() != scriptDialogs.Load() {
		return errors.New("真实脚本对话框的内置字体核对未全部通过；探针必须实际触发脚本对话框")
	}
	report.Checks = append(report.Checks, "真实脚本对话框正文和按钮使用同一内置 Eden UI 字体，并通过 GDI 回读核对")
	report.Checks = append(report.Checks, "真实 WebView2 localhost 工作台通过官方自定义 Transport 调用 DesktopService；无 /wails/runtime 后台请求")
	if _, err = os.Stat(filepath.Join(output, "Native_WebView2.png")); err != nil {
		return err
	}
	return nil
}
