package desktopupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/installfiles"
	"golang.org/x/sys/windows"
)

func writeCreationFailureProgram(t *testing.T, directory string, executable []byte, version string) {
	t.Helper()
	files := map[string][]byte{"Eden.exe": executable, "resources/eden.exe": []byte(version + " backend"), "Uninstall.exe": []byte(version + " uninstaller"), installfiles.IdentityName: []byte(installfiles.Identity)}
	manifest := installfiles.Manifest{Version: 1}
	for name, content := range files {
		writeTest(t, filepath.Join(directory, filepath.FromSlash(name)), string(content))
		hash := sha256.Sum256(content)
		manifest.Files = append(manifest.Files, installfiles.File{Path: name, SHA256: hex.EncodeToString(hash[:])})
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeTest(t, filepath.Join(directory, installfiles.ManifestName), string(data))
}

// This file-level regression uses a real CreateProcess format error, the
// production file-restore path and a real restored old process. It installs no
// certificates and does not invoke NSIS, a controller, or a VM on the host.
func TestCandidateCreationFailureRestoresAndReopensOriginal(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	oldImage, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"reopen-success", "reopen-failure"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			backup, target := filepath.Join(root, "previous-application"), filepath.Join(root, "installed application")
			originalBackup := backup
			writeCreationFailureProgram(t, backup, oldImage, "old")
			originalFiles := map[string]string{}
			for _, name := range []string{"Eden.exe", "resources/eden.exe", "Uninstall.exe", installfiles.IdentityName, installfiles.ManifestName} {
				originalFiles[name] = readTest(t, filepath.Join(backup, filepath.FromSlash(name)))
			}
			writeCreationFailureProgram(t, target, []byte("controlled invalid Windows PE image"), "new")
			profile := filepath.Join(root, "selected profile")
			config := filepath.Join(profile, "config.json")
			personal := map[string]string{config: `{"environment":"original-environment","account":"original-user","home":"/home/original-user"}`, filepath.Join(profile, "account.json"): "original login", filepath.Join(target, "my-project", "work.txt"): "latest personal work", filepath.Join(target, "EdenData", "persistent.img"): "original disk"}
			for name, content := range personal {
				writeTest(t, name, content)
			}
			disk, err := installfiles.OpenReadLocked(filepath.Join(target, "EdenData", "persistent.img"))
			if err != nil {
				t.Fatal(err)
			}
			defer disk.Close()
			plan := InstallPlan{Target: target, ConfigPath: config, Confirmation: newConfirmation()}
			t.Setenv("EDEN_CONFIG", filepath.Join(root, "unselected profile", "config.json"))
			t.Setenv("EDEN_UPDATE_CONFIRMATION", plan.Confirmation)
			candidate := exec.Command(filepath.Join(target, "Eden.exe"))
			hide(candidate)
			candidate.Dir = target
			candidate.Env = profileEnvironment(config, plan.Confirmation)
			started, exited, startErr := startTrackedApplication(candidate)
			if !errors.Is(startErr, windows.ERROR_BAD_EXE_FORMAT) && !errors.Is(startErr, windows.ERROR_EXE_MACHINE_TYPE_MISMATCH) {
				t.Fatal("expected a real CreateProcess image error", startErr)
			}
			if started != nil || exited != nil || candidate.Process != nil {
				t.Fatal("failed creation published process ownership")
			}
			if err := restoreApplicationFiles(context.Background(), plan, backup); err != nil {
				t.Fatal("old program recovery was not attempted successfully", err)
			}
			if actual, err := os.ReadFile(filepath.Join(target, "Eden.exe")); err != nil || !bytes.Equal(actual, oldImage) {
				t.Fatal("the original executable was not restored", err)
			}
			if _, err := os.Stat(filepath.Join(backup, "Eden.exe")); err != nil {
				t.Fatal("snapshot removed before original process creation", err)
			}
			marker := filepath.Join(profile, "reopened.json")
			old := exec.Command(filepath.Join(target, "Eden.exe"), "-test.run=^TestProcessLifecycleFixture$")
			hide(old)
			old.Dir = target
			old.Env = append(profileEnvironment(config, ""), "EDEN_UPDATE_TEST_PROCESS=recovered", "EDEN_UPDATE_TEST_MARKER="+marker)
			if mode == "reopen-failure" {
				old.Dir = filepath.Join(root, "missing working directory")
			}
			var oldExit chan error
			reopenErr := reopenRecovered(&backup, func() error {
				if _, err := os.Stat(filepath.Join(originalBackup, "Eden.exe")); err != nil {
					t.Fatal("reopen lost its recovery snapshot", err)
				}
				var err error
				_, oldExit, err = startTrackedApplication(old)
				return err
			})
			if mode == "reopen-failure" {
				if reopenErr == nil || old.Process != nil || oldExit != nil || backup != originalBackup {
					t.Fatal("real old-process creation failure was not retained", reopenErr)
				}
				if actual, err := os.ReadFile(filepath.Join(backup, "Eden.exe")); err != nil || !bytes.Equal(actual, oldImage) {
					t.Fatal("failed relaunch discarded the original backup", err)
				}
			} else {
				if reopenErr != nil {
					t.Fatal("restored original did not start", reopenErr)
				}
				select {
				case err := <-oldExit:
					if err != nil {
						t.Fatal("original process failed", err)
					}
				case <-time.After(5 * time.Second):
					_ = old.Process.Kill()
					<-oldExit
					t.Fatal("restored original did not finish")
				}
				var reopened struct{ Executable, Config, Confirmation string }
				if err := readSmallJSON(marker, &reopened, 16384); err != nil {
					t.Fatal("old application never reached its actual entry", err)
				}
				if !strings.EqualFold(reopened.Executable, filepath.Join(target, "Eden.exe")) || reopened.Config != config || reopened.Confirmation != "" {
					t.Fatal("recovery launched the wrong image/profile or inherited confirmation", reopened)
				}
				if backup != "" {
					t.Fatal("successful old-process creation did not complete backup cleanup")
				}
				if _, err := os.Stat(originalBackup); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("completed recovery retained the old snapshot", err)
				}
			}
			for name, want := range personal {
				if readTest(t, name) != want {
					t.Fatal("process creation recovery changed personal data", name)
				}
			}
			for name, want := range originalFiles {
				if got := readTest(t, filepath.Join(target, filepath.FromSlash(name))); got != want {
					t.Fatal("recovery changed original program inventory", name)
				}
				if mode == "reopen-failure" && readTest(t, filepath.Join(backup, filepath.FromSlash(name))) != want {
					t.Fatal("failed relaunch changed original backup inventory", name)
				}
			}
			if err := EnsureInstallationComplete(target); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStartedApplicationKeepsExactProcessAndWaitResult(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "started")
			cmd := exec.Command(executable, "-test.run=^TestProcessLifecycleFixture$")
			hide(cmd)
			cmd.Env = append(os.Environ(), "EDEN_UPDATE_TEST_PROCESS="+mode, "EDEN_UPDATE_TEST_MARKER="+marker)
			started, exited, err := startTrackedApplication(cmd)
			if err != nil {
				t.Fatal(err)
			}
			consumed := false
			defer func() {
				if !consumed {
					_ = cmd.Process.Kill()
					<-exited
				}
			}()
			if started != cmd || started.Process == nil || cap(exited) != 1 {
				t.Fatal("successful creation lost exact process or buffered Wait ownership")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := waitForLaunch(ctx, filepath.Join(root, "missing-confirmation.json"), "0.1.8", exited); err == nil || !strings.Contains(err.Error(), "目标进程已退出") {
				t.Fatal("exact candidate exit did not end readiness wait", err)
			}
			exitErr := <-exited
			consumed = true
			if mode == "success" && exitErr != nil {
				t.Fatal(exitErr)
			}
			if mode == "failure" {
				var failure *exec.ExitError
				if !errors.As(exitErr, &failure) || failure.ExitCode() != 7 {
					t.Fatal("single Wait result lost actual process exit status", exitErr)
				}
			}
			select {
			case <-exited:
				t.Fatal("process exit was published more than once")
			default:
			}
			if readTest(t, marker) != "started" {
				t.Fatal("the tracked process did not reach its real entry")
			}
		})
	}
}
