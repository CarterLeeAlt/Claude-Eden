package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/desktopupdate"
	"github.com/sylvinus/agent-vm/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// This VM-only host serves the unchanged production assets. The CDP driver
// supplies the isolated controller/RFB peer; Wails IPC and OS operations stay real.
func runNativeUISelfTest(output, scenario string, options Options) (report NativeSelfTestReport, err error) {
	machine, err := nativeTestMachine()
	if err != nil {
		return report, err
	}
	if scenario != "ui-session" && scenario != "ui-settings" && scenario != "ui-status" && scenario != "ui-input" && scenario != "ui-clipboard" {
		return report, errors.New("未知生产 UI 自检工况")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return report, err
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		return report, err
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		return report, errors.Join(err, errors.New("生产 UI 自检目录必须为空"))
	}
	report.Machine, _ = os.Hostname()
	report.VirtualMachine = machine
	report.Checks, report.Errors, report.Screenshots = []string{}, []string{}, []string{}
	defer func() {
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
		report.Success = len(report.Errors) == 0
		data, writeErr := json.MarshalIndent(report, "", "  ")
		if writeErr == nil {
			writeErr = os.WriteFile(filepath.Join(output, "Native_Result.json"), data, 0600)
		}
		err = errors.Join(err, writeErr)
	}()
	assets := os.Getenv("EDEN_NATIVE_UI_ASSETS")
	for _, name := range []string{"index.html", "build-integrity.json"} {
		if !filepath.IsAbs(assets) {
			return report, errors.New("生产 UI 资源目录必须是绝对路径")
		}
		if _, err = os.Stat(filepath.Join(assets, name)); err != nil {
			return report, err
		}
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return report, err
	}
	base := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	program, err := os.Executable()
	if err != nil {
		listener.Close()
		return report, err
	}
	options.BackendExecutable, options.ConfigPath, options.SmokeOutput = program, filepath.Join(output, "profile", "config.json"), ""
	shell, err := New(options)
	if err != nil {
		listener.Close()
		return report, err
	}
	defer shell.cancel()
	shell.selfTestBrowserArgs = []string{"--remote-debugging-port=19817", "--remote-debugging-address=127.0.0.1"}
	shell.backend.Directory = output
	shell.backend.prefix = []string{"--native-test-backend"}
	shell.backend.environment = []string{"EDEN_NATIVE_STARTUP_CASE=" + scenario, "EDEN_NATIVE_TEST_OUTPUT=" + output, "EDEN_NATIVE_TEST_ENTRY=" + base + "/#production-ui-ticket"}
	updates := filepath.Join(output, "profile", "updates")
	updater, err := desktopupdate.New(desktopupdate.Config{Root: updates, Target: filepath.Dir(program), Executable: program, Version: version.Version, ConfigPath: shell.backend.ConfigPath, Backend: shell.backend, Quit: shell.Quit})
	if err != nil {
		listener.Close()
		return report, err
	}
	defer updater.Close()
	if os.Getenv("EDEN_UPDATE_CONFIRMATION") != "" {
		listener.Close()
		return report, errors.New("保留已有升级启动确认，不运行 UI 自检")
	}
	token := uuid.NewString()
	if err = os.Setenv("EDEN_UPDATE_CONFIRMATION", token); err != nil {
		listener.Close()
		return report, err
	}
	defer os.Unsetenv("EDEN_UPDATE_CONFIRMATION")
	shell.options.MarkReady, shell.options.StartupFailed = updater.MarkReady, updater.MarkStartupFailed
	var traceMu sync.Mutex
	trace := []any{}
	appendTrace := func(value any) { traceMu.Lock(); defer traceMu.Unlock(); trace = append(trace, value) }
	traceRequest := func(request application.RuntimeRequest) any {
		return map[string]any{"object": request.Object, "method": request.Method, "args": json.RawMessage(request.Args.String()), "webviewWindowId": request.WebviewWindowID}
	}
	transport := &nativeTransport{port: func() int { return int(shell.expectedPort.Load()) }}
	transport.observe = func(request application.RuntimeRequest) {
		appendTrace(map[string]any{"phase": "dispatch", "utc": time.Now().UTC(), "request": traceRequest(request)})
	}
	transport.replied = func(request application.RuntimeRequest, callErr error) {
		shell.runtimeReplied(request, callErr)
		message := ""
		if callErr != nil {
			message = callErr.Error()
		}
		appendTrace(map[string]any{"phase": "replied", "utc": time.Now().UTC(), "request": traceRequest(request), "error": message})
	}
	shell.transport, shell.options.Transport = transport, transport
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	gate := make(chan struct{})
	var startOnce sync.Once
	secret := uuid.NewString()
	var clipboardMu sync.Mutex
	var restoreClipboard func() error
	var display atomic.Pointer[httputil.ReverseProxy]
	files := http.FileServer(http.Dir(assets))
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/__eden_test/") {
			if r.URL.Path == "/" {
				select {
				case <-gate:
				case <-ctx.Done():
					return
				}
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				if proxy := display.Load(); proxy != nil {
					proxy.ServeHTTP(w, r)
					return
				}
				http.Error(w, "Unmocked isolated controller request", http.StatusServiceUnavailable)
				return
			}
			files.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-Eden-Test") != secret || r.Method != http.MethodPost {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		var request struct{ Action, Text, Name, Owner string }
		if decodeErr := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&request); decodeErr != nil {
			http.Error(w, decodeErr.Error(), http.StatusBadRequest)
			return
		}
		var value any = true
		controlErr := application.InvokeSyncWithError(func() error {
			switch request.Action {
			case "start":
				startOnce.Do(func() { close(gate) })
			case "display":
				target, parseErr := url.Parse(request.Text)
				if parseErr != nil || target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.Port() == "" || target.User != nil {
					return errors.New("显示替身仅允许 VM 回环地址")
				}
				display.Store(httputil.NewSingleHostReverseProxy(target))
			case "state":
				marker, readErr := os.ReadFile(filepath.Join(updates, "launch-"+token+".json"))
				if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
					return readErr
				}
				owner := ""
				if shell.input != nil {
					owner = shell.input.owner
				}
				traceMu.Lock()
				calls := append([]any{}, trace...)
				traceMu.Unlock()
				value = map[string]any{"owner": owner, "visible": shell.window.IsVisible(), "ready": shell.ready.Load(), "startup": shell.startupResult.Load(), "generation": shell.clipboard.generation.Load(), "marker": string(marker), "calls": calls, "pageErrors": shell.smokeDiagnostics.pageErrors()}
			case "hide":
				shell.window.Hide()
				shell.publishVisibility()
			case "show":
				shell.restore()
			case "capture":
				if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(request.Name) {
					return errors.New("截图名称无效")
				}
				return captureVisibleNativeWindow(uintptr(shell.window.NativeWindow()), filepath.Join(output, request.Name+".png"))
			case "type":
				foreground, _, _ := getForegroundWindow.Call()
				focus, _, _ := getFocus.Call()
				if shell.input == nil || foreground != shell.input.parent || focus != shell.input.hwnd || shell.input.owner == "" {
					return errors.New("实际原生 EDIT 未持有自有焦点")
				}
				keys := []nativeKeyboardInput{}
				for _, character := range utf16.Encode([]rune(request.Text)) {
					keys = append(keys, nativeKeyboardInput{Type: 1, Scan: character, Flags: 4}, nativeKeyboardInput{Type: 1, Scan: character, Flags: 6})
				}
				if len(keys) == 0 {
					return errors.New("输入文本为空")
				}
				count, _, nativeErr := user32.NewProc("SendInput").Call(uintptr(len(keys)), uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0]))
				if count != uintptr(len(keys)) {
					return fmtNative("实际 UI Unicode 输入未完整发送", nativeErr)
				}
			case "key-chord":
				foreground, _, _ := getForegroundWindow.Call()
				focus, _, _ := getFocus.Call()
				if shell.input == nil || foreground != shell.input.parent || focus != shell.input.hwnd || shell.input.owner == "" {
					return errors.New("快捷键自检失去原生 EDIT 焦点")
				}
				key := uint16(0)
				switch request.Name {
				case "quick-open":
					key = 'P'
				case "sidebar":
					key = 'B'
				default:
					return errors.New("未知自检快捷键")
				}
				keys := [4]nativeKeyboardInput{{Type: 1, VK: 17}, {Type: 1, VK: key}, {Type: 1, VK: key, Flags: 2}, {Type: 1, VK: 17, Flags: 2}}
				if count, _, nativeErr := user32.NewProc("SendInput").Call(4, uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0])); count != 4 {
					return fmtNative("实际 UI 快捷键未完整发送", nativeErr)
				}
			case "old-input":
				if validID(request.Owner) != nil {
					return errors.New("旧 owner 无效")
				}
				shell.app.Event.Emit("eden-native-input", InputEvent{Owner: request.Owner, Kind: "input", Text: request.Text})
			case "clipboard-write":
				clipboardMu.Lock()
				defer clipboardMu.Unlock()
				if restoreClipboard == nil {
					var saveErr error
					restoreClipboard, saveErr = preserveUIClipboard(shell.clipboard.native)
					if saveErr != nil {
						return saveErr
					}
				}
				_, _, writeErr := shell.clipboard.native.write(clipboardSnapshot{text: request.Text}, nil)
				return writeErr
			case "clipboard-read":
				text, readErr := shell.clipboard.native.readText()
				value = text
				return readErr
			default:
				return errors.New("未知 UI 自检控制操作")
			}
			return nil
		})
		if controlErr != nil {
			http.Error(w, controlErr.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	connection, _ := json.Marshal(map[string]string{"base": base, "secret": secret, "scenario": scenario})
	if err = os.WriteFile(filepath.Join(output, "UI_Connection.json"), connection, 0600); err != nil {
		return report, err
	}
	done := make(chan error, 1)
	go func() {
		driver := exec.CommandContext(ctx, os.Getenv("EDEN_NATIVE_UI_NODE"), os.Getenv("EDEN_NATIVE_UI_DRIVER"), output, scenario)
		driver.Dir, driver.Env, driver.Stdout, driver.Stderr = output, os.Environ(), os.Stdout, os.Stderr
		hideProcess(driver)
		driverErr := driver.Run()
		driverErr = errors.Join(driverErr, application.InvokeSyncWithError(func() error {
			clipboardMu.Lock()
			defer clipboardMu.Unlock()
			if restoreClipboard != nil {
				return restoreClipboard()
			}
			return nil
		}))
		if driverErr == nil {
			pageCtx, pageCancel := context.WithTimeout(ctx, 3*time.Second)
			driverErr = shell.captureSmokePage(pageCtx)
			pageCancel()
		}
		done <- driverErr
		shell.Quit()
	}()
	if err = shell.Run(); err != nil {
		cancel()
		return report, err
	}
	err = <-done
	data, readErr := os.ReadFile(filepath.Join(output, "UI_Result.json"))
	if readErr == nil {
		var result struct{ Checks, Errors, Screenshots []string }
		readErr = json.Unmarshal(data, &result)
		report.Checks, report.Screenshots = result.Checks, result.Screenshots
		report.Errors = append(report.Errors, result.Errors...)
	}
	for _, failure := range shell.smokeDiagnostics.pageErrors() {
		report.Errors = append(report.Errors, startupPageMessage(failure))
	}
	traceMu.Lock()
	data, traceErr := json.MarshalIndent(trace, "", "  ")
	traceMu.Unlock()
	if traceErr == nil {
		traceErr = os.WriteFile(filepath.Join(output, "UI_Native_Calls.json"), data, 0600)
	}
	err = errors.Join(err, readErr, traceErr)
	return report, err
}

func preserveUIClipboard(clipboard nativeClipboard) (func() error, error) {
	type item struct {
		format uintptr
		data   []byte
	}
	var saved []item
	err := clipboard.opened(func() error {
		previous := uintptr(0)
		for {
			kernel32.NewProc("SetLastError").Call(0)
			format, _, nativeErr := user32.NewProc("EnumClipboardFormats").Call(previous)
			if format == 0 {
				if nativeErr != syscall.Errno(0) {
					return fmtNative("无法保存 VM 剪贴板", nativeErr)
				}
				return nil
			}
			previous = format
			if format != 1 && format != 7 && format != 8 && format != 13 && format != 15 && format != 16 && format != 17 && format != pngFormat() {
				return fmt.Errorf("VM 剪贴板含不可还原格式 %d", format)
			}
			data, readErr := clipboardBytes(format, maxPixels*4+4096)
			if readErr != nil {
				return readErr
			}
			saved = append(saved, item{format, data})
		}
	})
	if err != nil {
		return nil, err
	}
	return func() error {
		return clipboard.opened(func() error {
			if result, _, nativeErr := emptyClipboard.Call(); result == 0 {
				return fmtNative("无法恢复 VM 剪贴板", nativeErr)
			}
			for _, value := range saved {
				if err := putClipboard(value.format, value.data); err != nil {
					return err
				}
			}
			return nil
		})
	}, nil
}
