// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/portfwd"
	"github.com/lima-vm/sshocker/pkg/ssh"
)

func TestMain(m *testing.M) {
	mode := os.Getenv("EDEN_STARTUP_SSH_TEST")
	if mode == "" {
		os.Exit(m.Run())
	}
	if marker := os.Getenv("EDEN_STARTUP_SSH_MARKER"); marker != "" {
		if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(99)
		}
	}
	if mode == "blocked" {
		time.Sleep(20 * time.Second)
		os.Exit(99)
	}
	if code, err := strconv.Atoi(mode); err == nil {
		fmt.Fprint(os.Stderr, "startup fixture error")
		os.Exit(code)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func startupExitError(t *testing.T, code int) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "EDEN_STARTUP_SSH_TEST="+strconv.Itoa(code))
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != code {
		t.Fatalf("fixture exit: %v", err)
	}
	return err
}

func TestRequirementsWaitForReadinessWithoutAttemptLimit(t *testing.T) {
	pending := startupExitError(t, 1)
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		calls := 0
		err := waitForRequirements(t.Context(), "essential", []requirement{{description: "slow boot"}}, func(context.Context, requirement) error {
			calls++
			if calls <= 205 {
				return pending
			}
			return nil
		})
		if err != nil || calls != 206 || time.Since(started) <= 10*time.Minute {
			t.Fatalf("slow startup rejected: calls=%d elapsed=%s err=%v", calls, time.Since(started), err)
		}
	})
}

func TestRequirementsCancelAndPropagateFailures(t *testing.T) {
	pending, failed := startupExitError(t, 255), startupExitError(t, 125)
	for _, tc := range []struct {
		name  string
		err   error
		fatal bool
	}{
		{"local failure", errors.New("cannot execute SSH"), false},
		{"boot failure", failed, false},
		{"fatal probe", pending, true},
		{"SSH authentication", fmt.Errorf("Permission denied (publickey): %w", pending), false},
		{"SSH host key", fmt.Errorf("Host key verification failed: %w", pending), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := waitForRequirements(t.Context(), "essential", []requirement{{description: tc.name, fatal: tc.fatal}, {}}, func(context.Context, requirement) error { calls++; return tc.err })
			if !errors.Is(err, tc.err) || calls != 1 {
				t.Fatalf("failure lost or dependent work ran: calls=%d err=%v", calls, err)
			}
		})
	}
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		calls := 0
		go func() { time.Sleep(time.Second); cancel() }()
		err := waitForRequirements(ctx, "essential", []requirement{{}}, func(context.Context, requirement) error { calls++; return pending })
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("retry sleep did not cancel: calls=%d err=%v", calls, err)
		}
	})
}

type startupDriver struct {
	driver.Driver
	exited            chan error
	setupErr          error
	stop              func(context.Context) error
	connecting        chan struct{}
	connectionStopped chan struct{}
}

func (d *startupDriver) Start(context.Context) (chan error, error)   { return d.exited, nil }
func (d *startupDriver) AdditionalSetupForSSH(context.Context) error { return d.setupErr }
func (d *startupDriver) Info(context.Context) driver.Info            { return driver.Info{} }
func (d *startupDriver) Stop(ctx context.Context) error              { return d.stop(ctx) }

func (d *startupDriver) ForwardGuestAgent(context.Context) bool { return false }
func (d *startupDriver) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	select {
	case d.connecting <- struct{}{}:
	default:
	}
	<-ctx.Done()
	select {
	case d.connectionStopped <- struct{}{}:
	default:
	}
	return nil, "", ctx.Err()
}

type startupEvents struct {
	bytes.Buffer
	ready chan struct{}
}

func (w *startupEvents) Write(p []byte) (int, error) {
	var ev events.Event
	if err := json.Unmarshal(p, &ev); err != nil {
		return 0, err
	}
	if ev.Status.Running {
		select {
		case w.ready <- struct{}{}:
		default:
		}
	}
	return w.Buffer.Write(p)
}

func TestHostAgentStartupShutdownOrdering(t *testing.T) {
	bin := t.TempDir()
	exe := "ssh"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	image, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, exe), image, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, mode := range []string{"ready", "cancel", "signal", "driver-failure", "startup-failure", "setup-failure", "watcher-cancel"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "probe.pid")
			probeMode := "success"
			if mode == "cancel" || mode == "signal" || mode == "driver-failure" {
				probeMode = "blocked"
			}
			if mode == "startup-failure" {
				probeMode = "125"
			}
			t.Setenv("EDEN_STARTUP_SSH_TEST", probeMode)
			t.Setenv("EDEN_STARTUP_SSH_MARKER", marker)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			closed, stopped := false, false
			failure := errors.New("driver failed")
			d := &startupDriver{exited: make(chan error, 1), connecting: make(chan struct{}, 8), connectionStopped: make(chan struct{}, 8)}
			if mode == "setup-failure" {
				d.setupErr = failure
			}
			d.stop = func(ctx context.Context) error {
				if closed {
					return errors.New("shared files closed before guest shutdown")
				}
				if ctx.Err() != nil {
					return errors.New("guest shutdown inherited cancelled startup")
				}
				stopped = true
				return nil
			}
			out := &startupEvents{ready: make(chan struct{}, 1)}
			a := &HostAgent{
				instConfig: &limatype.LimaYAML{OS: new(limatype.LINUX), Plain: new(true), MountType: new(limatype.NINEP), SSH: limatype.SSH{ForwardAgent: new(false)}, HostResolver: limatype.HostResolver{Enabled: new(false)}, Containerd: limatype.Containerd{System: new(false), User: new(false)}},
				instDir:    dir, iid: "startup-test", driver: d, signalCh: make(chan os.Signal, 1), eventEnc: json.NewEncoder(out),
				sshConfig: &ssh.SSHConfig{},
			}
			if mode == "watcher-cancel" {
				a.instConfig.Plain = new(false)
				a.instConfig.MountInotify = new(true)
				a.guestAgentAliveCh = make(chan struct{})
				a.grpcPortForwarder = portfwd.NewPortForwarder(nil, true, true, nil)
			}
			a.cleanUp(func() error {
				closed = true
				if !stopped {
					return errors.New("cleanup preceded guest shutdown")
				}
				return nil
			})
			done := make(chan error, 1)
			go func() { done <- a.Run(ctx) }()
			if mode == "watcher-cancel" {
				select {
				case <-d.connecting:
					cancel()
				case err := <-done:
					t.Fatalf("startup failed before guest agent connection: %v", err)
				case <-time.After(5 * time.Second):
					cancel()
					<-done
					t.Fatal("guest agent watcher never started")
				}
			} else if probeMode == "blocked" {
				deadline := time.Now().Add(5 * time.Second)
				for {
					if _, err := os.Stat(marker); err == nil {
						break
					}
					if time.Now().After(deadline) {
						cancel()
						<-done
						t.Fatal("SSH fixture never started")
					}
					time.Sleep(10 * time.Millisecond)
				}
				switch mode {
				case "cancel":
					cancel()
				case "signal":
					a.signalCh <- os.Interrupt
				case "driver-failure":
					d.exited <- failure
				}
			} else if mode == "ready" {
				select {
				case <-out.ready:
					a.signalCh <- os.Interrupt
				case err := <-done:
					t.Fatalf("startup failed: %v", err)
				case <-time.After(5 * time.Second):
					cancel()
					<-done
					t.Fatal("startup did not become ready")
				}
			}
			select {
			case err := <-done:
				switch mode {
				case "ready", "signal":
					if err != nil {
						t.Fatal(err)
					}
				case "cancel", "watcher-cancel":
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancel lost: %v", err)
					}
				case "driver-failure", "setup-failure":
					if !errors.Is(err, failure) {
						t.Fatalf("driver failure lost: %v", err)
					}
				case "startup-failure":
					if err == nil || !strings.Contains(err.Error(), "125") {
						t.Fatalf("boot failure lost: %v", err)
					}
				}
			case <-time.After(5 * time.Second):
				cancel()
				<-done
				t.Fatal("shutdown did not join startup promptly")
			}
			if !closed || !stopped {
				t.Fatalf("resources not cleaned: stopped=%v closed=%v", stopped, closed)
			}
			if mode == "watcher-cancel" {
				if a.getClient() != nil {
					t.Fatal("guest client recreated after shutdown")
				}
				select {
				case <-d.connectionStopped:
				case <-time.After(time.Second):
					t.Fatal("guest connection did not close")
				}
			}
			if mode != "ready" && strings.Contains(out.String(), `"running":true`) {
				t.Fatal("failed/cancelled startup published running")
			}
			if raw, err := os.ReadFile(marker); err == nil {
				pid, err := strconv.Atoi(string(raw))
				if err != nil {
					t.Fatal(err)
				}
				if osutil.ProcessAlive(pid) {
					t.Fatalf("SSH probe %d still alive after Run", pid)
				}
			}
		})
	}
}
