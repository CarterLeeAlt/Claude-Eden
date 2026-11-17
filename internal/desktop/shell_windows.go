package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"html"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sylvinus/agent-vm/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type Options struct {
	BackendExecutable string
	ConfigPath        string
	Icon              []byte
	FontRegular       []byte
	FontBold          []byte
	SmokeOutput       string
	// WorkbenchHandler is installed only after origin/navigation security has
	// been configured. It may proxy the controller, not duplicate its VM logic.
	WorkbenchHandler http.Handler
	Transport        application.Transport
	Update           func(string) (UpdateState, error)
	MarkReady        func() error
	StartupFailed    func(error) error
	CloseUpdate      func()
	// Overrides the app's native policy installation for controlled test hosts.
	ConfigureWebViewPolicy func(*application.WebviewWindowOptions, WebViewPolicy) error
}

type WebViewPolicy struct {
	AllowNavigation     func(string) bool
	NavigationCompleted func(success bool, reason string)
	ProcessFailed       func(reason string)
}

type Shell struct {
	startupMu                                              sync.Mutex
	pageReadyMu                                            sync.Mutex
	nativeStart                                            sync.Once
	initialPageOnce                                        sync.Once
	initialPageDone                                        chan struct{}
	startupResult                                          atomic.Uint32
	options                                                Options
	backend                                                *Backend
	app                                                    *application.App
	window                                                 *application.WebviewWindow
	service                                                *DesktopService
	input                                                  *nativeInput
	startup                                                *nativeStatus
	fonts                                                  *nativeFonts
	menuTheme                                              *nativeMenuTheme
	webFailed                                              atomic.Bool
	started, restoreRequested                              atomic.Bool
	clipboard                                              *clipboardSync
	lifetime                                               context.Context
	cancel                                                 context.CancelFunc
	instance                                               *desktopInstance
	transport                                              *nativeTransport
	allowQuit, ready, controllerNeedsUpdate, bootstrapping atomic.Bool
	expectedPort                                           atomic.Int32
	smokeOnce                                              sync.Once
	smokeResult                                            *smokeResult
	smokeBrowsers                                          []smokeProcess
	smokeDiagnostics                                       smokeDiagnostics
	statusMu                                               sync.Mutex
	phase, status                                          string
	pageURL                                                string
	selfTestBrowserArgs                                    []string
}

func New(options Options) (*Shell, error) {
	backend, err := NewBackend(options.BackendExecutable, options.ConfigPath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Shell{options: options, backend: backend, lifetime: ctx, cancel: cancel, phase: "starting", initialPageDone: make(chan struct{})}
	s.service = &DesktopService{shell: s, selections: make(map[string]selectedFiles)}
	s.clipboard = &clipboardSync{backend: backend, lifetime: ctx, copies: make(map[string][]string), pending: make(map[string]bool), readers: make(map[string]selectedFiles)}
	return s, nil
}

func additionalBrowserArgsFromEnvironment() []string {
	value := strings.TrimSpace(os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"))
	if value == "" {
		return nil
	}
	return strings.Fields(value)
}

// Service exposes the concrete binding to the generator and to embedding hosts.
func (s *Shell) Service() *DesktopService { return s.service }
func (s *Shell) Backend() *Backend        { return s.backend }

func (s *Shell) Run() error {
	if s.options.ConfigureWebViewPolicy == nil {
		s.options.ConfigureWebViewPolicy = s.configureWebViewPolicy
	}
	if err := platformSupported(); err != nil {
		return err
	}
	fonts, err := newNativeFonts(s.options.FontRegular, s.options.FontBold)
	if err != nil {
		return err
	}
	s.fonts = fonts
	s.menuTheme = &nativeMenuTheme{fonts: fonts, background: application.NewRGB(247, 247, 247), foreground: application.NewRGB(26, 28, 31), dpi: 96}
	defer fonts.close()
	instance, second, err := acquireInstance(s.lifetime, s.backend.Root(), s.options.SmokeOutput != "", s.restore)
	if err != nil {
		return err
	}
	if second {
		if s.options.SmokeOutput != "" {
			return errors.New("自检配置已由另一个 Claude Eden 桌面进程使用")
		}
		return nil
	}
	s.instance = instance
	defer instance.close()
	defer s.cancel()
	if s.options.Transport == nil {
		s.transport = &nativeTransport{port: func() int { return int(s.expectedPort.Load()) }, replied: s.runtimeReplied}
		s.options.Transport = s.transport
	}
	assets := s.options.WorkbenchHandler
	if assets == nil {
		assets = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(LoadingHTML("正在启动 Claude Eden 工作台…")))
		})
	}
	browserArgs := append([]string(nil), s.selfTestBrowserArgs...)
	browserArgs = append(browserArgs, additionalBrowserArgsFromEnvironment()...)
	s.app = application.New(application.Options{
		Name: "Claude Eden", Description: "本机隔离编码工作区", Icon: s.options.Icon,
		Services: []application.Service{application.NewService(s.service)},
		Assets:   application.AssetOptions{Handler: assets, DisableLogging: true}, Transport: s.options.Transport,
		RawMessageHandler: func(window application.Window, message string, origin *application.OriginInfo) {
			if failures := s.smokeDiagnostics.receive(message, origin, int(s.expectedPort.Load())); len(failures) != 0 {
				s.pageStartupError(failures[0])
			}
			if s.transport != nil {
				s.transport.handle(window, message, origin)
			}
		},
		Windows:      application.WindowsOptions{WebviewUserDataPath: filepath.Join(s.backend.Root(), "webview2"), DisableQuitOnLastWindowClosed: true, WndProcInterceptor: s.intercept, AdditionalBrowserArgs: browserArgs},
		ErrorHandler: func(err error) { go s.backend.Report(err.Error()) },
		PanicHandler: func(details *application.PanicDetails) {
			go s.backend.Report(details.Error.Error() + "\n" + details.StackTrace)
			s.startupError("程序发生错误，后台任务保留。请重试连接或导出诊断包。")
			go func() {
				if s.window == nil {
					return
				}
				accepted, err := s.message("程序发生错误：\n"+details.Error.Error()+"\n\n是否退出 Claude Eden？", "Claude Eden 错误", true)
				if err == nil && accepted {
					s.Quit()
				}
			}()
		},
		OnShutdown: func() {
			s.started.Store(false)
			application.InvokeSync(s.savePlacement)
			s.cancel()
			if s.options.CloseUpdate != nil {
				s.options.CloseUpdate()
			}
			if s.input != nil {
				application.InvokeSync(s.input.close)
			}
			if s.startup != nil {
				application.InvokeSync(s.startup.close)
			}
		},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	windowOptions := application.WebviewWindowOptions{
		Name: "workbench", Title: "Claude Eden", Width: 1100, Height: 730, MinWidth: 820, MinHeight: 580,
		URL: "/", BackgroundColour: application.NewRGB(247, 247, 247), DefaultContextMenuDisabled: true, Zoom: 1,
		Windows: application.WindowsWindow{Theme: application.Light, DisableMenu: true},
		KeyBindings: map[string]func(application.Window){
			"Ctrl+O": func(application.Window) { s.action("openProject") }, "Ctrl+T": func(application.Window) { s.action("newTab") },
			"Ctrl+Shift+Enter": func(application.Window) { s.action("newTerminal") }, "Ctrl+W": func(application.Window) { s.action("closeTab") }, "Ctrl+,": func(application.Window) { s.action("settings") },
		},
	}
	if err := s.options.ConfigureWebViewPolicy(&windowOptions, WebViewPolicy{
		AllowNavigation: func(raw string) bool {
			return raw == "http://wails.localhost/" || IsWorkbench(raw, int(s.expectedPort.Load()))
		},
		NavigationCompleted: func(success bool, reason string) {
			if !success {
				s.startupError("打开工作台失败：" + reason + "。后台任务保留，可重试连接。")
			} else if s.expectedPort.Load() > 0 && s.startup != nil {
				application.InvokeAsync(s.startup.hide)
			}
			if s.expectedPort.Load() == 0 {
				s.initialPageOnce.Do(func() { close(s.initialPageDone) })
			}
		},
		ProcessFailed: func(reason string) {
			s.ready.Store(false)
			s.startupError("工作台显示进程已退出（" + reason + "）。后台任务保留，可重试连接。")
		},
	}); err != nil {
		return err
	}
	s.window = s.app.Window.NewWithOptions(windowOptions)
	s.window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		s.savePlacement()
		if !s.allowQuit.Load() {
			event.Cancel()
			s.window.Hide()
			s.publishVisibility()
		}
	})
	s.window.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) { s.app.Event.Emit("eden-window-deactivated") })
	for _, kind := range []events.WindowEventType{events.Common.WindowShow, events.Common.WindowHide, events.Common.WindowMinimise, events.Common.WindowUnMinimise} {
		s.window.OnWindowEvent(kind, func(*application.WindowEvent) { s.publishVisibility() })
	}
	s.setupTray()
	if err := s.app.Run(); err != nil {
		return err
	}
	return s.finishSmoke()
}

func (s *Shell) startNativeWindow() {
	if s.allowQuit.Load() {
		return
	}
	if err := application.InvokeSyncWithError(s.createNativeControls); err != nil {
		s.startupError(err.Error())
		return
	}
	s.started.Store(true)
	if s.restoreRequested.Swap(false) {
		s.restore()
	}
	if !s.webFailed.Load() {
		// Initialized means the controller exists, not that Wails' first page
		// finished navigating. Replacing that page early cancels its navigation
		// and reports a genuine OPERATION_CANCELED before the workbench loads.
		select {
		case <-s.initialPageDone:
		case <-s.lifetime.Done():
			return
		}
	}
	if s.webFailed.Load() || s.startupResult.Load() == startupFailed {
		s.statusMu.Lock()
		message := s.status
		s.statusMu.Unlock()
		application.InvokeSync(func() { s.startup.show(message, true, false) })
		return
	}
	go s.bootstrap("")
	if s.options.SmokeOutput == "" {
		return
	}
	go func() {
		select {
		case <-s.lifetime.Done():
		case <-time.After(120 * time.Second):
			s.completeSmoke(false, "工作台未在自检期限内确认就绪")
		}
	}()
}

func (s *Shell) createNativeControls() error {
	if err := s.restorePlacement(); err != nil {
		return err
	}
	input, err := newNativeInput(uintptr(s.window.NativeWindow()), func(value InputEvent) { s.app.Event.Emit("eden-native-input", value) })
	if err != nil {
		return err
	}
	s.input = input
	startup, err := newNativeStatus(uintptr(s.window.NativeWindow()), s.fonts)
	if err != nil {
		return err
	}
	s.startup = startup
	s.clipboard.native.hwnd = uintptr(s.window.NativeWindow())
	return nil
}

func (s *Shell) intercept(hwnd uintptr, message uint32, wParam, lParam uintptr) (uintptr, bool) {
	if result, handled := s.menuTheme.hook(hwnd, message, wParam, lParam); handled {
		return result, true
	}
	if s.window != nil && hwnd == uintptr(s.window.NativeWindow()) {
		if message == 0x0018 && wParam == 0 || message == 5 && wParam == 1 {
			if s.input != nil {
				s.input.deactivate()
			}
			s.clipboard.generation.Add(1)
		}
		if (message == 5 || message == 0x02e0) && s.startup != nil {
			s.startup.layout()
		}
		if message == 0x0111 && wParam>>16 == 0 {
			switch wParam & 0xffff {
			case 0x7101:
				go s.bootstrap("")
				return 0, true
			case 0x7102:
				go s.bootstrap("--existing")
				return 0, true
			case 0x7103:
				go func() {
					accepted, err := s.message("请先保存编辑器中的文件并结束正在运行的工作。继续后 Claude Eden 会停止环境、安装已验证的更新并重新启动。", "保存并停止环境后安装重启", true)
					if err != nil {
						s.errorStatus(err.Error())
					} else if accepted {
						go s.bootstrap("--complete-update")
					}
				}()
				return 0, true
			}
		}
	}
	// QUERYENDSESSION is only a request. Windows can cancel it with
	// ENDSESSION(FALSE), in which case close-to-tray must remain unchanged.
	if message == 0x0011 {
		return 1, true
	}
	if message == 0x0016 {
		if wParam != 0 {
			s.Quit()
		}
		return 0, true
	}
	return 0, false
}

func (s *Shell) restore() {
	s.restoreRequested.Store(true)
	if !s.started.Load() {
		return
	}
	s.restoreRequested.Store(false)
	if s.window != nil {
		s.window.Show()
		if s.window.IsMinimised() {
			s.window.UnMinimise()
		}
		s.window.Focus()
		s.publishVisibility()
	}
}
func (s *Shell) action(name string) { s.app.Event.Emit("eden-native-action", name) }
func (s *Shell) publishVisibility() {
	if s.window != nil {
		s.app.Event.Emit("eden-window-visibility", s.window.IsVisible() && !s.window.IsMinimised())
	}
}
func (s *Shell) PublishUpdate(state UpdateState) {
	if s.app != nil {
		s.app.Event.Emit("eden-app-update", state)
	}
	if state.Phase == "failed" {
		go s.backend.Report("App update failed: " + state.Message)
	}
}
func (s *Shell) Quit() {
	if err := s.reportStartupFailed(context.Canceled); err != nil {
		go s.backend.Report("无法报告启动取消：" + err.Error())
	}
	s.allowQuit.Store(true)
	if s.app != nil {
		s.app.Quit()
	}
}
func (s *Shell) Reconnect() { go s.bootstrap("") }

func (s *Shell) setupTray() {
	menu := application.NewMenu()
	menu.Add("打开 Claude Eden").OnClick(func(*application.Context) { s.restore() })
	menu.Add("检查应用更新…").OnClick(func(*application.Context) {
		if s.options.Update != nil {
			if _, err := s.options.Update("check"); err != nil {
				s.errorStatus(err.Error())
			}
		}
	})
	menu.Add("关于 Claude Eden").OnClick(func(*application.Context) {
		_, err := s.message("Claude Eden "+version.Version+"\n本机隔离编码工作区", "关于 Claude Eden", false)
		if err != nil {
			s.errorStatus(err.Error())
		}
	})
	menu.AddSeparator()
	menu.Add("退出 Claude Eden…").OnClick(func(*application.Context) {
		s.restore()
		if s.ready.Load() {
			s.action("quit")
		} else {
			s.Quit()
		}
	})
	tray := s.app.SystemTray.New().SetMenu(menu).OnDoubleClick(s.restore)
	tray.SetTooltip("Claude Eden")
	if len(s.options.Icon) > 0 {
		tray.SetIcon(s.options.Icon)
	}
}

func (s *Shell) bootstrap(option string) {
	if !s.bootstrapping.CompareAndSwap(false, true) {
		return
	}
	defer s.bootstrapping.Store(false)
	if s.webFailed.Load() {
		if err := s.window.RecoverWebView(); err != nil {
			s.startupError("无法重新创建工作台显示组件：" + err.Error())
			return
		}
		s.webFailed.Store(false)
	}
	s.setStatus("starting_backend", "正在连接 Claude Eden 工作台…")
	if s.startup != nil {
		application.InvokeSync(func() { s.startup.show("正在连接 Claude Eden 工作台…", false, false) })
	}
	args := []string{"desktop", "--json"}
	if option != "" {
		args = append(args, option)
	}
	var reply bootstrapReply
	code, err := s.backend.JSON(s.lifetime, 0, 65536, &reply, args...)
	if err == nil && code != 0 {
		if reply.Message == "" {
			reply.Message = "工作台未能启动"
		}
		err = errors.New(reply.Message)
	}
	if err != nil {
		if s.lifetime.Err() == nil {
			s.startupError(err.Error())
		}
		return
	}
	s.controllerNeedsUpdate.Store(reply.Status != "ready")
	if reply.Status == "upgrade_pending" {
		s.setStatus("upgrade_pending", reply.Message)
		if s.startup != nil {
			application.InvokeSync(func() { s.startup.show(reply.Message, false, true) })
		}
		s.app.Event.Emit("eden-startup-state", map[string]any{"phase": "upgrade_pending", "message": reply.Message, "canOpenExisting": true, "canCompleteUpdate": true})
		return
	}
	entry, err := WorkbenchEntry(reply.URL)
	if err != nil {
		s.startupError(err.Error())
		return
	}
	s.expectedPort.Store(int32(portOf(entry)))
	s.statusMu.Lock()
	safeEntry := *entry
	safeEntry.RawQuery = ""
	safeEntry.Fragment = ""
	s.pageURL = safeEntry.String()
	s.statusMu.Unlock()
	s.setStatus("loading_workbench", "正在打开 Claude Eden 工作台…")
	if s.controllerNeedsUpdate.Load() {
		s.window.SetTitle("Claude Eden（更新待完成）")
	} else {
		s.window.SetTitle("Claude Eden")
	}
	s.window.SetURL(entry.String())
}

func (s *Shell) setStatus(phase, message string) {
	if phase == "failed" {
		s.ready.Store(false)
	}
	s.statusMu.Lock()
	s.phase, s.status = phase, message
	s.statusMu.Unlock()
}
func (s *Shell) errorStatus(message string) {
	s.setStatus("failed", message)
	if s.startup != nil {
		application.InvokeAsync(func() { s.startup.show(message, true, false) })
	}
	go s.backend.Report(message)
	if s.app != nil {
		s.app.Event.Emit("eden-startup-state", map[string]any{"phase": "failed", "message": message, "retryable": true})
	}
}

func encodeClipboardImage(value clipboardSnapshot) ClipboardContent {
	return ClipboardContent{PNG: base64.StdEncoding.EncodeToString(value.png)}
}

func (s *Shell) completeSmoke(ready bool, failure string) {
	if s.options.SmokeOutput == "" {
		return
	}
	s.smokeOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			errorsFound := []string{}
			if failure != "" {
				errorsFound = append(errorsFound, failure)
			}
			pageContext, pageCancel := context.WithTimeout(ctx, 5*time.Second)
			if err := s.captureSmokePage(pageContext); err != nil {
				errorsFound = append(errorsFound, err.Error())
			}
			pageCancel()
			pageErrors := s.smokeDiagnostics.pageErrors()
			if len(pageErrors) != 0 {
				errorsFound = append(errorsFound, "工作台 JavaScript 检测到错误，详见 pageErrors")
			}
			controllers, captureError := s.backend.captureSmokeControllers()
			if captureError != nil {
				errorsFound = append(errorsFound, captureError.Error())
			}
			defer closeSmokeProcesses(controllers)
			var prepared bootstrapReply
			code, err := s.backend.JSON(ctx, 20*time.Second, 65536, &prepared, "prepare-update", "--json")
			if err != nil {
				errorsFound = append(errorsFound, err.Error())
			} else if code != 0 || prepared.Status != "prepared" {
				errorsFound = append(errorsFound, "自检结束时无法确认后台已停止")
			}
			browsers, err := captureSmokeBrowsers()
			if err != nil {
				errorsFound = append(errorsFound, err.Error())
			}
			s.statusMu.Lock()
			phase, status, pageURL := s.phase, s.status, s.pageURL
			s.statusMu.Unlock()
			if phase == "failed" {
				errorsFound = append(errorsFound, redactMessage(status))
			}
			report := &smokeResult{NativeBridgeReady: ready, Version: version.Version, Phase: phase, NativeStatus: status, PrepareExit: code, PrepareStatus: prepared.Status, PrepareMessage: redactMessage(prepared.Message), Errors: errorsFound, PageErrors: pageErrors, PageURL: pageURL}
			application.InvokeSync(func() {
				bounds := s.window.PhysicalBounds()
				report.WindowBounds = Rectangle{bounds.X, bounds.Y, bounds.Width, bounds.Height}
				if screen, err := s.window.GetScreen(); err == nil {
					work := screen.PhysicalWorkArea
					report.WorkingArea = Rectangle{work.X, work.Y, work.Width, work.Height}
				}
				dpi, _, _ := getDPIForWindow.Call(uintptr(s.window.NativeWindow()))
				report.DeviceDPI = int(dpi)
				if value, ok := s.app.Env.Info().PlatformInfo["WebView2"].(string); ok {
					report.WebviewVersion = value
				}
				if !ready || len(report.Errors) != 0 {
					if err := captureNativeWindow(uintptr(s.window.NativeWindow()), smokeScreenshotPath(s.options.SmokeOutput)); err != nil {
						report.Errors = append(report.Errors, "无法保存工作台画面："+err.Error())
					}
				}
			})
			s.smokeResult = report
			s.smokeBrowsers = browsers
			s.Quit()
		}()
	})
}

// LoadingHTML is host-owned and escapes diagnostic text rather than executing it.
func LoadingHTML(message string) string {
	return "<!doctype html><meta charset=utf-8><title>Claude Eden</title><p>" + html.EscapeString(message) + "</p>"
}
