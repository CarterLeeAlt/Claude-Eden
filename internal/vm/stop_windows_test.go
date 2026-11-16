package vm

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/sylvinus/agent-vm/internal/host"
	"golang.org/x/sys/windows"
)

func TestWindowsHostagentPIDFixture(t *testing.T) {
	if os.Getenv("EDEN_VM_STOP_PID_FIXTURE") != "1" {
		t.Skip("subprocess fixture")
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

func hostagentPIDFixture(t *testing.T) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsHostagentPIDFixture$")
	cmd.Env = append(os.Environ(), "EDEN_VM_STOP_PID_FIXTURE=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		input.Close()
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
		cancel()
	})
	return cmd.Process.Pid
}

func TestWindowsStopWaitsForHostagentAfterDriverExit(t *testing.T) {
	for _, operation := range []string{"stop", "delete", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			l := newTestLima(t)
			none := []Mount{}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := l.Create(ctx, "ending", "template:debian-13", Settings{Mounts: &none}, io.Discard); err != nil {
				t.Fatal(err)
			}
			inst, err := l.inspect(ctx, "ending")
			if err != nil {
				t.Fatal(err)
			}
			pidfile := filepath.Join(inst.Dir, filenames.HostAgentPID)
			if err := store.WritePIDFile(pidfile, hostagentPIDFixture(t)); err != nil {
				t.Fatal(err)
			}
			inst, err = l.inspect(ctx, "ending")
			if err != nil || inst.Status != limatype.StatusBroken || inst.HostAgentPID == 0 || inst.DriverPID != 0 {
				t.Fatalf("expected driver-exited, hostagent-alive transition: %+v, %v", inst, err)
			}
			notified := make(chan struct{})
			closeListener, err := host.ListenInstanceStop(inst.Dir, func() { close(notified) })
			if err != nil {
				t.Fatal(err)
			}
			defer closeListener()
			// The workbench can notify the owner before the stop command starts.
			if sent, err := host.RequestInstanceStop(inst.Dir); err != nil || !sent {
				t.Fatalf("initial stop notification: %v, %v", sent, err)
			}
			select {
			case <-notified:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			done := make(chan error, 1)
			go func() {
				if operation == "delete" {
					done <- l.Delete(ctx, "ending")
				} else {
					done <- l.Stop(ctx, "ending")
				}
			}()
			select {
			case err := <-done:
				t.Fatalf("returned before the notified hostagent finished: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			if operation == "cancel" {
				cancel()
				if err := <-done; !errors.Is(err, context.Canceled) {
					t.Fatalf("stop wait did not preserve cancellation: %v", err)
				}
				if _, err := os.Stat(pidfile); err != nil {
					t.Fatalf("cancelled wait altered hostagent state: %v", err)
				}
				return
			}
			if err := os.Remove(pidfile); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			current, err := l.Get(ctx, "ending")
			if operation == "delete" {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("instance still exists after completed deletion: %v", err)
				}
			} else if err != nil || current.Status != Stopped {
				t.Fatalf("stop did not confirm the final state: %+v, %v", current, err)
			}
		})
	}
}

func TestWindowsStopRejectsMissingHostagentNotification(t *testing.T) {
	l := newTestLima(t)
	none := []Mount{}
	ctx := context.Background()
	if err := l.Create(ctx, "unowned", "template:debian-13", Settings{Mounts: &none}, io.Discard); err != nil {
		t.Fatal(err)
	}
	inst, err := l.inspect(ctx, "unowned")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WritePIDFile(filepath.Join(inst.Dir, filenames.HostAgentPID), hostagentPIDFixture(t)); err != nil {
		t.Fatal(err)
	}
	if err := l.Stop(ctx, "unowned"); err == nil {
		t.Fatal("an unowned live hostagent was reported as stopped")
	}
}
