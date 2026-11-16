package instance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostAgentStartupWaitsForPIDWithoutElapsedFailure(t *testing.T) {
	dir := t.TempDir()
	pid := filepath.Join(dir, "ha.pid")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- waitHostAgentStart(ctx, pid, filepath.Join(dir, "stderr.log"), nil) }()
	select {
	case err := <-done:
		t.Fatalf("slow startup was rejected: %v", err)
	case <-time.After(5200 * time.Millisecond):
	}
	if err := os.WriteFile(pid, []byte("123"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHostAgentStartupFailureAndCancellation(t *testing.T) {
	for _, kind := range []string{"cancel", "exit-success", "exit-failure", "invalid-path"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			pid := filepath.Join(dir, "ha.pid")
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			exited := make(chan error, 1)
			failure := errors.New("fixture process failed")
			switch kind {
			case "cancel":
				cancel()
			case "exit-success":
				exited <- nil
			case "exit-failure":
				exited <- failure
			case "invalid-path":
				if err := os.Mkdir(pid, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err := waitHostAgentStart(ctx, pid, filepath.Join(dir, "stderr.log"), exited)
			if err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("did not report the actual failure: %v", err)
			}
			if kind == "cancel" && !errors.Is(err, context.Canceled) || kind == "exit-failure" && !errors.Is(err, failure) || kind == "exit-success" && !strings.Contains(err.Error(), "exited") {
				t.Fatalf("lost failure cause: %v", err)
			}
		})
	}
}
