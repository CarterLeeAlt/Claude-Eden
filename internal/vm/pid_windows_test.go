package vm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lima-vm/lima/v2/pkg/store"
	"golang.org/x/sys/windows"
)

func TestPIDExitFixture(t *testing.T) {
	if code := os.Getenv("EDEN_PID_EXIT_FIXTURE"); code != "" {
		n, err := strconv.Atoi(code)
		if err != nil {
			t.Fatal(err)
		}
		os.Exit(n)
	}
}

func TestLimaPIDFileExitedProcessWithRetainedHandle(t *testing.T) {
	for _, code := range []int{0, 259} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			image, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, image, "-test.run=^TestPIDExitFixture$")
			cmd.Env = append(os.Environ(), "EDEN_PID_EXIT_FIXTURE="+strconv.Itoa(code))
			// cmd retains its process handle until Wait; add another one before
			// reaping it to reproduce an exited but still openable process.
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
			if err != nil {
				_ = cmd.Wait()
				t.Fatal(err)
			}
			defer windows.CloseHandle(h)
			err = cmd.Wait()
			if code == 0 && err != nil {
				t.Fatal(err)
			}
			if code != 0 && cmd.ProcessState.ExitCode() != code {
				t.Fatalf("exit = %v, state = %v", err, cmd.ProcessState)
			}
			file := filepath.Join(t.TempDir(), "ha.pid")
			if err := store.WritePIDFile(file, cmd.Process.Pid); err != nil {
				t.Fatal(err)
			}
			pid, err := store.ReadPIDFile(file)
			if err != nil || pid != 0 {
				t.Fatalf("exited PID reported alive: pid=%d error=%v", pid, err)
			}
			if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("stale PID file remains: %v", err)
			}
		})
	}
}

func TestLimaPIDFileLiveProcessRetained(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ha.pid")
	if err := store.WritePIDFile(file, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	pid, err := store.ReadPIDFile(file)
	if err != nil || pid != os.Getpid() {
		t.Fatalf("live PID = %d, error = %v", pid, err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
}
