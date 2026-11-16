// Headless system-integration fixture. It uses the production transaction;
// only release transport and workbench readiness are local, deterministic data.
// Run solely in the approved disposable Windows VM via test-windows-update-flow.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/appupdate"
	"github.com/sylvinus/agent-vm/internal/desktopupdate"
	"github.com/sylvinus/agent-vm/internal/installfiles"
)

var fixtureVersion = "development"

type settings struct {
	Mode      string `json:"mode"`
	Installer string `json:"installer"`
}
type fixtureBackend struct {
	profile, executable string
	settings            settings
	cancel              func()
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func record(profile, name string, value any) error {
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(profile, name+".json"), b, 0600)
}
func waitRelease(profile string) error {
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if _, e := os.Stat(filepath.Join(profile, "release.json")); e == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("fixture release deadline exceeded")
}
func (b *fixtureBackend) Update(ctx context.Context, check bool, emit func(appupdate.State)) error {
	if check {
		emit(appupdate.State{Phase: "available", LatestVersion: "0.1.8"})
		return nil
	}
	root := filepath.Join(b.profile, "updates")
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	job, e := os.MkdirTemp(root, "download-")
	if e != nil {
		return e
	}
	archive := filepath.Join(job, "Eden-setup.exe")
	input, e := os.Open(b.settings.Installer)
	if e != nil {
		return e
	}
	defer input.Close()
	output, e := os.OpenFile(archive, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	hash := sha256.New()
	_, e = io.Copy(io.MultiWriter(output, hash), input)
	e = errors.Join(e, output.Close())
	if e != nil {
		return e
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if b.settings.Mode == "digest-failure" {
		digest = strings.Repeat("0", 64)
	}
	latestVersion := "0.1.8"
	if b.settings.Mode == "metadata-version-failure" {
		latestVersion = "0.1.9"
	}
	emit(appupdate.State{Phase: "downloaded", CurrentVersion: fixtureVersion, LatestVersion: latestVersion, ArchivePath: archive, SHA256: digest})
	return ctx.Err()
}
func (b *fixtureBackend) PrepareUpdate(ctx context.Context, stop bool) error {
	if b.settings.Mode == "prepare-failure" {
		return errors.New("fixture environment preparation refused")
	}
	if b.settings.Mode == "prepare-cancel" && b.cancel != nil {
		b.cancel()
		return ctx.Err()
	}
	if b.settings.Mode == "stop-required" && !stop {
		return desktopupdate.ErrStopRequired
	}
	return record(b.profile, "handoff-"+fixtureVersion, map[string]any{"profile": b.profile, "version": fixtureVersion, "executable": b.executable, "stop": stop})
}

func run() error {
	config, e := filepath.Abs(os.Getenv("EDEN_CONFIG"))
	if e != nil {
		return e
	}
	profile := filepath.Dir(config)
	if e = os.MkdirAll(profile, 0700); e != nil {
		return e
	}
	if e = os.Setenv("EDEN_CONFIG", config); e != nil {
		return e
	}
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	root := filepath.Join(profile, "updates")
	if e = os.MkdirAll(root, 0700); e != nil {
		return e
	}
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "--install-update" {
		working, e := os.Getwd()
		if e != nil {
			return e
		}
		ticks, e := desktopupdate.CurrentStartTicks()
		if e != nil {
			return e
		}
		if e = record(profile, "helper", map[string]any{"pid": os.Getpid(), "directory": working, "executable": executable, "startTicks": ticks}); e != nil {
			return e
		}
		e = desktopupdate.RunInstallerWithDiagnostics(context.Background(), root, executable, os.Stdin, os.Stdout, filepath.Join(profile, "update-installer.json"))
		exit := 0
		if e != nil {
			exit = 1
		}
		if save := record(profile, "helper-finished", map[string]int{"exit": exit}); save != nil {
			return save
		}
		return e
	}
	var options settings
	data, e := os.ReadFile(filepath.Join(profile, "fixture.json"))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(data, &options); e != nil {
		return e
	}
	backend := &fixtureBackend{profile: profile, executable: executable, settings: options}
	if len(args) == 2 && args[0] == "--probe-helper" {
		return probeHelper(profile, config, executable, args[1], backend)
	}
	if len(args) == 2 && args[0] == "--hold-directory" {
		held, e := installfiles.OpenReadLocked(filepath.Join(args[1], installfiles.IdentityName))
		if e != nil {
			return e
		}
		defer held.Close()
		if e = record(profile, "held", map[string]int{"pid": os.Getpid()}); e != nil {
			return e
		}
		return waitRelease(profile)
	}
	if len(args) > 0 && args[0] == "prepare-update" {
		if e = backend.PrepareUpdate(context.Background(), contains(args, "--stop")); e != nil {
			return e
		}
		_, e = fmt.Fprintln(os.Stdout, `{"status":"prepared"}`)
		return e
	}
	configOptions := desktopupdate.Config{Root: root, Target: filepath.Dir(executable), Executable: executable, Version: fixtureVersion, ConfigPath: config, Backend: backend}
	if len(args) == 1 && args[0] == "--start-update" {
		failed := make(chan struct{}, 1)
		configOptions.Changed = func(s desktopupdate.State) {
			if s.Phase == "failed" {
				_ = record(profile, "parent-failed", s)
				select {
				case failed <- struct{}{}:
				default:
				}
			}
		}
		configOptions.Quit = func() { os.Exit(0) }
		configOptions.Reconnect = func() { _ = record(profile, "reconnected", map[string]bool{"reconnected": true}) }
		updater, e := desktopupdate.New(configOptions)
		if e != nil {
			return e
		}
		defer updater.Close()
		backend.cancel = func() { _, _ = updater.Handle("cancel") }
		if _, e = updater.Handle("install"); e != nil {
			return e
		}
		select {
		case <-failed:
			return errors.New("fixture update rejected")
		case <-time.After(3 * time.Minute):
			return errors.New("fixture parent timed out")
		}
	}
	if len(args) != 0 {
		return errors.New("unknown update fixture operation")
	}
	if fixtureVersion == "0.1.8" {
		startupStarted := time.Now()
		ticks, e := desktopupdate.CurrentStartTicks()
		if e != nil {
			return e
		}
		if e = record(profile, "candidate", map[string]any{"version": fixtureVersion, "profile": profile, "pid": os.Getpid(), "startTicks": ticks}); e != nil {
			return e
		}
		if options.Mode == "startup-failure" {
			if e = os.WriteFile(filepath.Join(filepath.Dir(executable), "my-project", "work.txt"), []byte("saved by new version"), 0600); e != nil {
				return e
			}
			return errors.New("fixture startup failure")
		}
		if options.Mode == "startup-wrong-ack" {
			token := os.Getenv("EDEN_UPDATE_CONFIRMATION")
			if e = record(root, "launch-"+token, map[string]string{"version": "0.1.7"}); e != nil {
				return e
			}
			return waitRelease(profile)
		}
		updater, e := desktopupdate.New(configOptions)
		if e != nil {
			return e
		}
		defer updater.Close()
		if options.Mode == "startup-error" || options.Mode == "startup-cancel" {
			cause := errors.New("fixture live controller startup failure")
			if options.Mode == "startup-cancel" {
				cause = context.Canceled
			}
			if e = updater.MarkStartupFailed(cause); e != nil {
				return e
			}
			if e = record(profile, "failure-reported-live", map[string]any{"pid": os.Getpid(), "startTicks": ticks, "utc": time.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
				return e
			}
			// Do not exit: only the production helper's failure acknowledgement
			// handling may stop this live candidate and recover the original.
			return waitRelease(profile)
		}
		if options.Mode == "startup-slow-success" {
			time.Sleep(185 * time.Second)
		}
		if e = updater.MarkReady(); e != nil {
			return e
		}
		if e = record(profile, "active", map[string]any{"version": fixtureVersion, "startupSeconds": time.Since(startupStarted).Seconds(), "profile": profile}); e != nil {
			return e
		}
	} else {
		ticks, e := desktopupdate.CurrentStartTicks()
		if e != nil {
			return e
		}
		if e = record(profile, "reopened", map[string]any{"version": fixtureVersion, "profile": profile, "pid": os.Getpid(), "startTicks": ticks}); e != nil {
			return e
		}
	}
	return waitRelease(profile)
}

// probeHelper exercises native authentication and the reversible protocol with
// real signed files. It never commits or invokes NSIS.
func probeHelper(profile, config, executable, target string, backend *fixtureBackend) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var state appupdate.State
	if e := backend.Update(ctx, false, func(s appupdate.State) { state = s }); e != nil {
		return e
	}
	publisher, e := desktopupdate.Publisher(executable)
	if e != nil {
		return e
	}
	ticks, e := desktopupdate.CurrentStartTicks()
	if e != nil {
		return e
	}
	confirmation := "12345678-1234-4234-8234-123456789abc"
	plan := desktopupdate.InstallPlan{Archive: state.ArchivePath, Digest: state.SHA256, Version: state.LatestVersion, Publisher: publisher, Target: target, ParentPID: os.Getpid(), ParentStartTicks: ticks, Confirmation: confirmation, ConfigPath: config}
	if backend.settings.Mode == "parent-start-mismatch" {
		plan.ParentStartTicks++
	}
	if backend.settings.Mode == "parent-pid-invalid" {
		plan.ParentPID = -1
	}
	root := filepath.Join(profile, "updates")
	helper := filepath.Join(root, "probe-helper.exe")
	input, e := os.Open(executable)
	if e != nil {
		return e
	}
	output, e := os.OpenFile(helper, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		input.Close()
		return e
	}
	_, e = io.Copy(output, input)
	e = errors.Join(e, input.Close(), output.Close())
	if e != nil {
		return e
	}
	defer os.Remove(helper)
	cmd := exec.Command(helper, "--install-update")
	cmd.Dir = root
	cmd.Env = os.Environ()
	stdin, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	defer stdout.Close()
	if e = cmd.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		stdin.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	stop := context.AfterFunc(ctx, func() { stdin.Close(); stdout.Close(); _ = cmd.Process.Kill() })
	defer stop()
	if e = json.NewEncoder(stdin).Encode(plan); e != nil {
		return e
	}
	reader := bufio.NewReader(stdout)
	line, readErr := reader.ReadString('\n')
	reject := strings.HasPrefix(backend.settings.Mode, "parent-")
	if reject {
		stdin.Close()
		exitErr := <-done
		done <- exitErr
		if readErr == nil || strings.TrimSpace(line) == "READY" || exitErr == nil {
			return errors.New("helper accepted invalid parent identity")
		}
		var rejected struct {
			Message string `json:"message"`
		}
		data, e := os.ReadFile(filepath.Join(root, "last-result.json"))
		if e != nil {
			return e
		}
		if e = json.Unmarshal(data, &rejected); e != nil {
			return e
		}
		expected := "更新请求进程身份不匹配"
		if backend.settings.Mode == "parent-pid-invalid" {
			expected = "更新请求进程标识无效"
		}
		if !strings.Contains(rejected.Message, expected) {
			return fmt.Errorf("parent identity probe failed before the intended check: %s", rejected.Message)
		}
		return record(profile, "probe-finished", map[string]any{"rejected": true, "mode": backend.settings.Mode})
	}
	if readErr != nil || strings.TrimSpace(line) != "READY" {
		return fmt.Errorf("helper did not become READY: %q %v", line, readErr)
	}
	// Verified archive must reject both in-place writes and path replacement.
	writeDenied := false
	if f, e := os.OpenFile(state.ArchivePath, os.O_WRONLY, 0600); e != nil {
		writeDenied = true
	} else {
		f.Close()
	}
	renameDenied := os.Rename(state.ArchivePath, state.ArchivePath+".replaced") != nil
	if !writeDenied || !renameDenied {
		return errors.New("READY archive lock allowed a writer or replacer")
	}
	if e = record(profile, "probe-ready", map[string]any{"writeDenied": writeDenied, "renameDenied": renameDenied, "helperPid": cmd.Process.Pid}); e != nil {
		return e
	}
	if backend.settings.Mode == "ready-disconnect" {
		// Exit the exact parent process without COMMIT; Windows closes the pipe.
		os.Exit(0)
	}
	if backend.settings.Mode == "ready-invalid-command" {
		if _, e = io.WriteString(stdin, "CANCEL\n"); e != nil {
			return e
		}
	}
	stdin.Close()
	exitErr := <-done
	done <- exitErr
	if exitErr == nil {
		return errors.New("revoked preparation returned success")
	}
	return record(profile, "probe-finished", map[string]any{"revoked": true, "mode": backend.settings.Mode})
}
func contains(args []string, want string) bool {
	for _, v := range args {
		if v == want {
			return true
		}
	}
	return false
}
