package desktopupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testConfirmation = "12345678-1234-4234-8234-123456789abc"

func TestStartupConfirmationTerminalResults(t *testing.T) {
	for _, mode := range []string{"ready", "failure", "cancel", "empty-error", "large-error"} {
		t.Run(mode, func(t *testing.T) {
			u := testUpdater(t, &fakeBackend{}, nil)
			t.Setenv("EDEN_UPDATE_CONFIRMATION", testConfirmation)
			cause := errors.New("controlled backend failure")
			switch mode {
			case "cancel":
				cause = context.Canceled
			case "empty-error":
				cause = errors.New(" ")
			case "large-error":
				cause = errors.New(strings.Repeat("\x00错误", 20000))
			}
			var err error
			if mode == "ready" {
				err = u.MarkReady()
			} else {
				err = u.MarkStartupFailed(cause)
			}
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(u.config.Root, "launch-"+testConfirmation+".json")
			var record launchConfirmation
			if err := readSmallJSON(name, &record, 16384); err != nil {
				t.Fatal(err)
			}
			if record.Version != u.config.Version || (record.Error == "") != (mode == "ready") {
				t.Fatal(record)
			}
			if mode == "cancel" && !strings.Contains(record.Error, "已取消") {
				t.Fatal(record)
			}
			before, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("EDEN_UPDATE_CONFIRMATION") != "" {
				t.Fatal("terminal acknowledgement retained its token")
			}
			if err := u.MarkReady(); err != nil {
				t.Fatal(err)
			}
			if err := u.MarkStartupFailed(errors.New("late failure")); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("late callback replaced a terminal result")
			}
		})
	}
}

func TestStartupFailureUnpublishableUsesSafeExit(t *testing.T) {
	u := testUpdater(t, &fakeBackend{}, nil)
	t.Setenv("EDEN_UPDATE_CONFIRMATION", testConfirmation)
	if err := os.WriteFile(u.config.Root, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	quit := 0
	u.config.Quit = func() { quit++; _ = u.Status() }
	if err := u.MarkStartupFailed(errors.New("real failure")); err == nil {
		t.Fatal("write failure swallowed")
	}
	if quit != 1 {
		t.Fatal("failed candidate did not use safe exit", quit)
	}
	if os.Getenv("EDEN_UPDATE_CONFIRMATION") != testConfirmation {
		t.Fatal("failed publication consumed token")
	}
}

func TestWaitForLaunchTerminalRecords(t *testing.T) {
	for _, tc := range []struct{ name, contents, failure string }{
		{"ready", `{"version":"0.1.8"}`, ""},
		{"failure", `{"version":"0.1.8","error":"controller failed"}`, "controller failed"},
		{"wrong-version", `{"version":"0.1.7"}`, "版本不匹配"},
		{"missing-version", `{}`, "版本不匹配"},
		{"invalid-json", `{"version":`, "无法读取"},
		{"oversized", strings.Repeat(" ", 16385), "超过允许大小"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "launch.json")
			if err := os.WriteFile(name, []byte(tc.contents), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := waitForLaunch(ctx, name, "0.1.8", make(chan error, 1))
			if tc.failure == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.failure) {
				t.Fatal(err)
			}
		})
	}
}

func TestWaitForLaunchRetainsExitForRecovery(t *testing.T) {
	name := filepath.Join(t.TempDir(), "launch.json")
	exited := make(chan error, 1)
	exitErr := errors.New("controlled exit")
	exited <- exitErr
	if err := waitForLaunch(context.Background(), name, "0.1.8", exited); err == nil || !strings.Contains(err.Error(), "目标进程已退出") {
		t.Fatal(err)
	}
	select {
	case got := <-exited:
		if !errors.Is(got, exitErr) {
			t.Fatal(got)
		}
	default:
		t.Fatal("recovery lost exact-process exit")
	}
}

func TestWaitForLaunchIgnoresParentDeadline(t *testing.T) {
	name := filepath.Join(t.TempDir(), "launch.json")
	parent, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	<-parent.Done()
	done := make(chan error, 1)
	exited := make(chan error, 1)
	t.Cleanup(func() {
		select {
		case exited <- errors.New("test cleanup"):
		default:
		}
	})
	go func() { done <- waitForLaunch(context.WithoutCancel(parent), name, "0.1.8", exited) }()
	select {
	case err := <-done:
		t.Fatal("parent deadline ended startup", err)
	case <-time.After(550 * time.Millisecond):
	}
	if err := writeResultFile(name, launchConfirmation{Version: "0.1.8"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ready confirmation not observed")
	}
}

func TestWaitForLaunchHonorsExplicitLocalCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForLaunch(ctx, filepath.Join(t.TempDir(), "launch.json"), "0.1.8", make(chan error, 1)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
