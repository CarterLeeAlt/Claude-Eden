package desktop

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestProductionDocumentFailureBeforeAndAfterConfirmation(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		for _, kind := range []string{"error", "module-load", "unhandledrejection"} {
			t.Run(kind+map[bool]string{false: " before ready", true: " after ready"}[confirmed], func(t *testing.T) {
				root := t.TempDir()
				// Error reporting cannot start a real backend or touch native UI.
				shell := &Shell{backend: &Backend{Executable: filepath.Join(root, "absent-backend.exe"), Directory: root, ConfigPath: filepath.Join(root, "config.json")}}
				readyCalls, failureCalls := 0, 0
				shell.options.MarkReady = func() error { readyCalls++; return nil }
				shell.options.StartupFailed = func(cause error) error {
					failureCalls++
					if !strings.Contains(cause.Error(), kind) {
						t.Fatal("page failure lost its kind")
					}
					return nil
				}
				service := &DesktopService{shell: shell}
				if confirmed {
					if _, err := service.WorkbenchReady(); err != nil {
						t.Fatal(err)
					}
				}
				message := smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: kind, Source: "/assets/entry.js", Message: "controlled page failure"}})
				fresh := shell.smokeDiagnostics.receive(message, smokeDiagnosticOrigin(), 7654)
				if len(fresh) != 1 {
					t.Fatal("trusted document failure was not received")
				}
				shell.pageStartupError(fresh[0])
				if confirmed {
					if failureCalls != 0 || !shell.ready.Load() || shell.phase == "failed" || shell.startupResult.Load() != startupReady {
						t.Fatal("ordinary late page error replaced successful startup")
					}
					return
				}
				if failureCalls != 1 || readyCalls != 0 || shell.startupResult.Load() != startupFailed || shell.phase != "failed" {
					t.Fatal("production page failure did not complete the pending startup")
				}
				if _, err := service.WorkbenchReady(); err != nil {
					t.Fatal("later UI recovery failed", err)
				}
				if readyCalls != 0 || shell.startupResult.Load() != startupFailed || !shell.ready.Load() {
					t.Fatal("UI recovery replaced the startup terminal failure")
				}
			})
		}
	}
}

func TestSlowProductionDocumentRemainsPendingWithoutAnError(t *testing.T) {
	shell := &Shell{}
	if fresh := shell.smokeDiagnostics.receive(smokeDiagnosticMessage(t, uuid.NewString(), "", nil), smokeDiagnosticOrigin(), 7654); len(fresh) != 0 {
		t.Fatal("clean loading page generated a failure")
	}
	if shell.startupResult.Load() != startupPending {
		t.Fatal("loading was mistaken for a terminal result")
	}
}

func TestStartupConfirmationCannotOvertakeAlreadyCollectedPageFailure(t *testing.T) {
	shell := &Shell{}
	shell.smokeDiagnostics.receive(smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Message: "fixture"}}), smokeDiagnosticOrigin(), 7654)
	if err := shell.markStartupReady(); err == nil || shell.startupResult.Load() != startupFailed {
		t.Fatal("page failure was overwritten by readiness")
	}
}

func TestStartupTerminalCallbacksAreSerializedAndReentrantFailureQuitIsSafe(t *testing.T) {
	for range 50 {
		shell := &Shell{}
		var mu sync.Mutex
		var callbacks []string
		shell.options.MarkReady = func() error { mu.Lock(); callbacks = append(callbacks, "ready"); mu.Unlock(); return nil }
		shell.options.StartupFailed = func(error) error {
			shell.Quit()
			mu.Lock()
			callbacks = append(callbacks, "failed")
			mu.Unlock()
			return nil
		}
		var group sync.WaitGroup
		group.Add(2)
		go func() { defer group.Done(); _ = shell.markStartupReady() }()
		go func() { defer group.Done(); _, _ = shell.failStartup(errors.New("fixture failure")) }()
		group.Wait()
		if len(callbacks) != 1 || shell.startupResult.Load() == startupPending {
			t.Fatal("racing startup callbacks published multiple terminal results", callbacks)
		}
	}
}

func TestReadinessPublicationFailureIsTerminalBeforeTheCallerReturns(t *testing.T) {
	shell := &Shell{}
	want := errors.New("startup marker write failed")
	readyCalls, failureCalls := 0, 0
	shell.options.MarkReady = func() error { readyCalls++; return want }
	shell.options.StartupFailed = func(cause error) error {
		failureCalls++
		if !errors.Is(cause, want) {
			t.Error("readiness publication failure lost its cause")
		}
		shell.Quit()
		return nil
	}
	if err := shell.markStartupReady(); !errors.Is(err, want) {
		t.Fatal("readiness publication error was hidden", err)
	}
	if shell.startupResult.Load() != startupFailed || failureCalls != 1 {
		t.Fatal("failed confirmation left a gap for a concurrent success")
	}
	if err := shell.markStartupReady(); err != nil || readyCalls != 1 {
		t.Fatal("late readiness retried a terminal confirmation", err)
	}
}

func TestWorkbenchFailedValidatesAndPreservesTheFirstStartupResult(t *testing.T) {
	for _, message := range []string{"", " \n\t ", strings.Repeat("x", 4097), strings.Repeat("中", 1366), string([]byte{0xff}), "fixture\x00failure"} {
		service := &DesktopService{shell: &Shell{}}
		if accepted, err := service.WorkbenchFailed(message); accepted || err == nil || service.shell.startupResult.Load() != startupPending {
			t.Fatal("invalid startup failure message changed the terminal result")
		}
	}
	for _, ready := range []bool{false, true} {
		root := t.TempDir()
		shell := &Shell{backend: &Backend{Executable: filepath.Join(root, "absent.exe"), Directory: root, ConfigPath: filepath.Join(root, "config.json")}}
		calls := 0
		shell.options.StartupFailed = func(cause error) error {
			calls++
			if strings.Contains(cause.Error(), "private-token") || strings.Contains(cause.Error(), "external.example") {
				t.Error("unredacted startup failure reached the updater")
			}
			return nil
		}
		service := &DesktopService{shell: shell}
		if ready {
			if _, err := service.WorkbenchReady(); err != nil {
				t.Fatal(err)
			}
		}
		accepted, err := service.WorkbenchFailed("controlled failure token=private-token https://external.example/private")
		if err != nil || accepted == ready || calls != map[bool]int{false: 1, true: 0}[ready] {
			t.Fatal("typed startup failure replaced an existing terminal result", err)
		}
		if accepted, err := service.WorkbenchFailed(strings.Repeat("x", 4096)); accepted || err != nil {
			t.Fatal("duplicate valid failure changed the terminal result", err)
		}
	}
}
