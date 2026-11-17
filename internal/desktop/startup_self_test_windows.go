package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/desktopupdate"
	"github.com/sylvinus/agent-vm/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// RunNativeStartupSelfTest runs the ordinary Shell.Run path (not smoke) with a
// controlled localhost page and the real updater confirmation writer. Its only
// deadline belongs to this explicitly VM-only acceptance entry.
func RunNativeStartupSelfTest(output, scenario string, options Options) (report NativeSelfTestReport, err error) {
	if strings.HasPrefix(scenario, "ui-") {
		return runNativeUISelfTest(output, scenario, options)
	}
	machine, err := nativeTestMachine()
	if err != nil {
		return report, err
	}
	if scenario != "slow" && scenario != "before-error-ready" && scenario != "before-explicit" && scenario != "after-explicit" && scenario != "before-error" && scenario != "before-module-load" && scenario != "before-unhandledrejection" && scenario != "after-error" && scenario != "after-module-load" && scenario != "after-unhandledrejection" {
		return report, errors.New("未知生产启动自检工况")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return report, err
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		return report, err
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		return report, err
	}
	if len(entries) != 0 {
		return report, errors.New("生产启动自检目录必须为空")
	}
	report.Machine, _ = os.Hostname()
	report.VirtualMachine = machine
	report.Errors, report.Checks, report.Screenshots = []string{}, []string{}, []string{}
	defer func() {
		if _, pictureError := os.Stat(filepath.Join(output, "Native_Startup.png")); pictureError == nil {
			report.Screenshots = append(report.Screenshots, "Native_Startup.png")
		}
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
		report.Success = len(report.Errors) == 0
		data, writeError := json.MarshalIndent(report, "", "  ")
		if writeError == nil {
			writeError = os.WriteFile(filepath.Join(output, "Native_Result.json"), data, 0600)
		}
		err = errors.Join(err, writeError)
	}()
	module, err := os.ReadFile(os.Getenv("EDEN_NATIVE_STARTUP_MODULE"))
	if err != nil {
		return report, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return report, err
	}
	observed := make(chan struct{})
	var observedOnce sync.Once
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html lang="zh-CN" data-scenario="%s"><meta charset="utf-8"><title>Claude Eden 生产启动自检</title><h1>生产启动回执：%s</h1><p>受控页面，不启动真实账户或客机。</p><script type="module" src="/startup-probe.js"></script></html>`, scenario, scenario)
		case "/startup-probe.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(module)
		case "/observed":
			observedOnce.Do(func() { close(observed) })
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	program, err := os.Executable()
	if err != nil {
		return report, err
	}
	options.BackendExecutable, options.ConfigPath, options.SmokeOutput = program, filepath.Join(output, "profile", "config.json"), ""
	shell, err := New(options)
	if err != nil {
		return report, err
	}
	defer shell.cancel()
	shell.backend.Directory = output
	shell.backend.prefix = []string{"--native-test-backend"}
	shell.backend.environment = []string{"EDEN_NATIVE_STARTUP_CASE=" + scenario, "EDEN_NATIVE_TEST_OUTPUT=" + output, "EDEN_NATIVE_TEST_ENTRY=http://localhost:" + fmt.Sprint(listener.Addr().(*net.TCPAddr).Port) + "/#startup-self-test"}
	updates := filepath.Join(output, "profile", "updates")
	updater, err := desktopupdate.New(desktopupdate.Config{Root: updates, Target: filepath.Dir(program), Executable: program, Version: version.Version, ConfigPath: shell.backend.ConfigPath, Backend: shell.backend, Quit: shell.Quit})
	if err != nil {
		return report, err
	}
	defer updater.Close()
	token := uuid.NewString()
	if os.Getenv("EDEN_UPDATE_CONFIRMATION") != "" {
		return report, errors.New("保留已有升级启动确认，不运行启动自检")
	}
	if err = os.Setenv("EDEN_UPDATE_CONFIRMATION", token); err != nil {
		return report, err
	}
	defer os.Unsetenv("EDEN_UPDATE_CONFIRMATION")
	shell.options.MarkReady, shell.options.StartupFailed = updater.MarkReady, updater.MarkStartupFailed
	var navigated atomic.Bool
	shell.options.ConfigureWebViewPolicy = func(window *application.WebviewWindowOptions, policy WebViewPolicy) error {
		completed := policy.NavigationCompleted
		policy.NavigationCompleted = func(success bool, reason string) {
			if success && shell.expectedPort.Load() > 0 {
				navigated.Store(true)
			}
			completed(success, reason)
		}
		return shell.configureWebViewPolicy(window, policy)
	}
	budget := 30 * time.Second
	if scenario == "slow" {
		budget = 160 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		observationError := observeStartupSelfTest(ctx, shell, observed, &navigated, filepath.Join(updates, "launch-"+token+".json"), scenario)
		if observationError != nil {
			fmt.Fprintf(os.Stderr, "Startup fixture observation failed: %v; nativeStarted=%t; expectedPort=%d; startupResult=%d\n", observationError, shell.started.Load(), shell.expectedPort.Load(), shell.startupResult.Load())
		}
		captureError := application.InvokeSyncWithError(func() error {
			if shell.window == nil || shell.window.NativeWindow() == nil {
				return errors.New("自检未创建真实工作台窗口")
			}
			return captureVisibleNativeWindow(uintptr(shell.window.NativeWindow()), filepath.Join(output, "Native_Startup.png"))
		})
		done <- errors.Join(observationError, captureError)
		shell.Quit()
	}()
	if err = shell.Run(); err != nil {
		cancel()
		return report, err
	}
	if err = <-done; err != nil {
		return report, err
	}
	report.Checks = append(report.Checks, "真实 Shell.Run、受信文档早期诊断与实际 npm runtime/生成绑定；SmokeOutput 为空", "主文档导航成功后按真实页面状态处理启动回执："+scenario)
	if scenario == "slow" {
		report.Checks = append(report.Checks, "无页面错误、未确认就绪的慢页面持续 125 秒仍为 pending；自检显式取消后退出")
	}
	return report, nil
}

func observeStartupSelfTest(ctx context.Context, shell *Shell, observed <-chan struct{}, navigated *atomic.Bool, marker, scenario string) error {
	select {
	case <-ctx.Done():
		shell.statusMu.Lock()
		status := shell.status
		shell.statusMu.Unlock()
		return fmt.Errorf("受控页面尚未执行到故障注入：%w；原生状态：%s；页面诊断：%v", ctx.Err(), status, shell.smokeDiagnostics.pageErrors())
	case <-observed:
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	started := time.Now()
	var initial []byte
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		if !navigated.Load() {
			continue
		}
		data, err := os.ReadFile(marker)
		if scenario == "slow" {
			if !errors.Is(err, os.ErrNotExist) || shell.startupResult.Load() != startupPending || len(shell.smokeDiagnostics.pageErrors()) != 0 {
				return errors.New("正常慢加载被提前写成启动终态")
			}
			if time.Since(started) >= 125*time.Second {
				return nil
			}
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var result struct {
			Version string `json:"version"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		if result.Version != version.Version {
			return errors.New("启动回执版本不匹配")
		}
		if strings.HasPrefix(scenario, "before-") {
			if result.Error == "" || shell.startupResult.Load() != startupFailed {
				return errors.New("首次确认前的生产页面错误未形成失败回执")
			}
			if !strings.Contains(result.Error, strings.TrimSuffix(strings.TrimPrefix(scenario, "before-"), "-ready")) {
				return fmt.Errorf("启动失败回执丢失错误类型：%s", result.Error)
			}
			return nil
		}
		if result.Error != "" || shell.startupResult.Load() != startupReady || !shell.ready.Load() {
			return errors.New("已确认成功被后续页面错误覆盖")
		}
		if initial == nil {
			initial = data
		}
		if !bytes.Equal(initial, data) {
			return errors.New("后续页面错误重写了成功回执")
		}
		if (len(shell.smokeDiagnostics.pageErrors()) != 0 || scenario == "after-explicit") && time.Since(started) >= time.Second {
			return nil
		}
	}
}
