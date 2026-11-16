package reversesshfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
)

func TestMain(m *testing.M) {
	mode := os.Getenv("REVERSESSHFS_STARTUP_TEST")
	if mode == "" {
		os.Exit(m.Run())
	}
	marker := os.Getenv("REVERSESSHFS_STARTUP_MARKER")
	if slices.Contains(os.Args, "mkdir") {
		if mode == "prepare-cancel" {
			time.Sleep(30 * time.Second)
		}
		os.Exit(0)
	}
	if slices.Contains(os.Args, "sshfs") {
		if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(99)
		}
		if mode == "transport-failure" {
			os.Exit(7)
		}
		time.Sleep(90 * time.Second)
		os.Exit(99)
	}
	if _, err := io.ReadAll(os.Stdin); err != nil {
		os.Exit(98)
	}
	if mode == "probe-failure" {
		fmt.Fprint(os.Stderr, "mount table unavailable")
		os.Exit(125)
	}
	if mode == "cancel" || mode == "transport-failure" {
		time.Sleep(30 * time.Second)
		os.Exit(99)
	}
	info, err := os.Stat(marker)
	if err != nil || (mode == "slow" && time.Since(info.ModTime()) < 31*time.Second) {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestMountStartupReadinessAndCleanup(t *testing.T) {
	bin := t.TempDir()
	name := "ssh"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	image, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, name), image, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, mode := range []string{"ready", "slow", "cancel", "transport-failure", "probe-failure", "prepare-cancel"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("REVERSESSHFS_STARTUP_TEST", mode)
			t.Setenv("REVERSESSHFS_STARTUP_MARKER", filepath.Join(root, "transport.pid"))
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			if mode == "cancel" || mode == "prepare-cancel" {
				ctx, cancel = context.WithTimeout(ctx, time.Second)
				defer cancel()
			}
			rsf := &ReverseSSHFS{SSHConfig: &ssh.SSHConfig{}, Driver: DriverBuiltin, Host: "fixture", LocalPath: root, RemotePath: "/fixture"}
			if err := rsf.Prepare(ctx); err != nil {
				if mode == "prepare-cancel" && errors.Is(err, context.DeadlineExceeded) {
					return
				}
				t.Fatal(err)
			}
			if mode == "prepare-cancel" {
				t.Fatal("prepare ignored cancellation")
			}
			started := time.Now()
			err := rsf.Start(ctx)
			switch mode {
			case "ready", "slow":
				if err != nil {
					t.Fatal(err)
				}
				if mode == "slow" && time.Since(started) < 31*time.Second {
					t.Fatal("mount published before readiness")
				}
				cancel()
				select {
				case <-rsf.sshDone:
					t.Fatal("startup cancellation closed an established mount")
				default:
				}
				if err := rsf.Close(); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("cancel lost: %v", err)
				}
			case "transport-failure":
				if err == nil || !strings.Contains(err.Error(), "exit status 7") {
					t.Fatalf("transport exit lost: %v", err)
				}
			case "probe-failure":
				if err == nil || !strings.Contains(err.Error(), "mount table unavailable") {
					t.Fatalf("probe failure lost: %v", err)
				}
			}
			select {
			case <-rsf.sshDone:
			default:
				t.Fatal("SSH transport not joined")
			}
			select {
			case <-rsf.serverDone:
			default:
				t.Fatal("SFTP server not joined")
			}
			if rsf.sshCmd.ProcessState == nil {
				t.Fatal("SSH transport not reaped")
			}
		})
	}
}
