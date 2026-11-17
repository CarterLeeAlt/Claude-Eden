package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sylvinus/agent-vm/internal/desktopupdate"
)

// A shell with no application/window exercises only state transitions. Never
// send actual session messages or create a window from host unit tests.
func TestCancelledWindowsSessionPreservesCloseToTray(t *testing.T) {
	shell := &Shell{}
	for _, message := range []struct {
		kind   uint32
		value  uintptr
		result uintptr
	}{{0x0011, 0, 1}, {0x0016, 0, 0}} {
		result, handled := shell.intercept(0, message.kind, message.value, 0)
		if !handled || result != message.result || shell.allowQuit.Load() {
			t.Fatal("queried/cancelled Windows session changed normal close-to-tray")
		}
	}
	if result, handled := shell.intercept(0, 0x0016, 1, 0); !handled || result != 0 || !shell.allowQuit.Load() {
		t.Fatal("committed Windows session did not request normal application shutdown")
	}
}

func TestSecondInstanceRestoreIsRetainedDuringStartup(t *testing.T) {
	shell := &Shell{}
	shell.restore()
	if !shell.restoreRequested.Load() {
		t.Fatal("restore request was lost before the window started")
	}
}

func TestFailedWorkbenchDoesNotDelegateTrayQuitToThePage(t *testing.T) {
	shell := &Shell{}
	shell.ready.Store(true)
	shell.setStatus("failed", "fixture")
	if shell.ready.Load() || shell.phase != "failed" {
		t.Fatal("failed page still marked ready for frontend-owned tray quit")
	}
}

func TestStartupFailureCallbackPreservesCauseAndAllowsUpdaterSafeExit(t *testing.T) {
	shell := &Shell{}
	want := errors.New("real startup failure")
	publishError := errors.New("marker could not be written")
	calls := 0
	shell.options.StartupFailed = func(cause error) error {
		calls++
		if cause != want {
			t.Fatal("desktop replaced the startup failure cause")
		}
		// The real updater takes this path when it cannot publish the marker.
		// Re-entering Quit must not deadlock or recursively notify cancellation.
		shell.Quit()
		return publishError
	}
	if err := shell.reportStartupFailed(want); err != publishError || calls != 1 || !shell.allowQuit.Load() {
		t.Fatalf("failure reporting/exit changed: %v, callbacks=%d", err, calls)
	}
}

func TestExplicitQuitReportsPendingStartupCancellationButReadinessIsTerminal(t *testing.T) {
	for _, ready := range []bool{false, true} {
		shell := &Shell{}
		calls := 0
		shell.options.StartupFailed = func(cause error) error {
			calls++
			if !errors.Is(cause, context.Canceled) {
				t.Fatal("explicit quit lost its cancellation cause")
			}
			return nil
		}
		if ready {
			if accepted, err := (&DesktopService{shell: shell}).WorkbenchReady(); err != nil || !accepted {
				t.Fatal("workbench readiness failed", err)
			}
			// A later page failure resets the tray-ready bit, not the startup
			// terminal result already accepted by the independent update helper.
			shell.setStatus("failed", "later renderer failure")
		}
		shell.Quit()
		if calls != map[bool]int{false: 1, true: 0}[ready] || !shell.allowQuit.Load() {
			t.Fatal("quit changed startup terminal acknowledgement")
		}
	}
}

func TestDesktopStartupCallbacksPublishRealUpdaterTerminalMarkers(t *testing.T) {
	const confirmation = "12345678-1234-4234-8234-123456789abc"
	for _, mode := range []string{"ready", "failure", "cancel", "marker-write-failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("EDEN_UPDATE_CONFIRMATION", confirmation)
			root := t.TempDir()
			backend, err := NewBackend(filepath.Join(root, "unused-backend.exe"), filepath.Join(root, "config.json"))
			if err != nil {
				t.Fatal(err)
			}
			shell := &Shell{backend: backend}
			updates := filepath.Join(root, "updates")
			updater, err := desktopupdate.New(desktopupdate.Config{Root: updates, Target: root, Executable: filepath.Join(root, "Eden.exe"), Version: "0.1.8", ConfigPath: backend.ConfigPath, Backend: backend, Quit: shell.Quit})
			if err != nil {
				t.Fatal(err)
			}
			defer updater.Close()
			shell.options.MarkReady, shell.options.StartupFailed = updater.MarkReady, updater.MarkStartupFailed
			service := &DesktopService{shell: shell}
			if mode == "marker-write-failure" {
				if err := os.WriteFile(updates, []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := shell.reportStartupFailed(errors.New("fixture startup failure")); err == nil || !shell.allowQuit.Load() {
					t.Fatal("unpublishable failure did not cause the real updater's safe exit", err)
				}
				return
			}
			switch mode {
			case "ready":
				if ready, err := service.WorkbenchReady(); err != nil || !ready {
					t.Fatal("readiness acknowledgement failed", err)
				}
			case "failure":
				if err := shell.reportStartupFailed(errors.New("fixture startup failure")); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				shell.Quit()
			}
			marker := filepath.Join(updates, "launch-"+confirmation+".json")
			before, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ Version, Error string }
			if json.Unmarshal(before, &result) != nil || result.Version != "0.1.8" || (result.Error == "") != (mode == "ready") {
				t.Fatalf("desktop callback published the wrong terminal result: %s", before)
			}
			if mode == "ready" {
				if err := shell.reportStartupFailed(errors.New("late failure")); err != nil {
					t.Fatal(err)
				}
			} else {
				if ready, err := service.WorkbenchReady(); err != nil || !ready {
					t.Fatal("late page readiness changed its service result", err)
				}
			}
			after, err := os.ReadFile(marker)
			if err != nil || string(after) != string(before) {
				t.Fatal("a late callback replaced the terminal marker", err)
			}
		})
	}
}
