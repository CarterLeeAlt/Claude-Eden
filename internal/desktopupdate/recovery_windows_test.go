package desktopupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/appupdate"
	"github.com/sylvinus/agent-vm/internal/installfiles"
	"golang.org/x/sys/windows"
)

func writeTest(t *testing.T, name, content string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(name), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(name, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}
func readTest(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func TestProgramSnapshotAndOverlap(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeTest(t, filepath.Join(source, "program.txt"), "program")
	writeTest(t, filepath.Join(source, "my-project", "work.txt"), "personal")
	writeTest(t, filepath.Join(source, installfiles.ManifestName), `{"version":1,"files":[{"path":"program.txt","sha256":"`+strings.Repeat("a", 64)+`"}]}`)
	if e := CopyTree(context.Background(), source, target); e != nil {
		t.Fatal(e)
	}
	if readTest(t, filepath.Join(target, "program.txt")) != "program" {
		t.Fatal("missing program")
	}
	if _, e := os.Stat(filepath.Join(target, "my-project")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("snapshot copied live user data")
	}
	nested := filepath.Join(source, "updates", "backup")
	if e := CopyTree(context.Background(), source, nested); e == nil {
		t.Fatal("accepted overlapping snapshot")
	}
	if _, e := os.Stat(nested); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("created overlapping output before validation")
	}
}

func TestOriginalIntactClassification(t *testing.T) {
	root := t.TempDir()
	target, backup := filepath.Join(root, "target"), filepath.Join(root, "backup")
	for _, dir := range []string{target, backup} {
		writeTest(t, filepath.Join(dir, "Eden.exe"), "original launcher")
		writeTest(t, filepath.Join(dir, installfiles.IdentityName), installfiles.Identity)
		writeTest(t, BackendPath(dir), "original backend")
	}
	writeTest(t, filepath.Join(target, "my-project", "work.txt"), "latest user data")
	if !OriginalIntact(target, backup) {
		t.Fatal("unchanged original rejected")
	}
	writeTest(t, BackendPath(target), "modified backend")
	if OriginalIntact(target, backup) {
		t.Fatal("same launcher hid backend change")
	}
	writeTest(t, BackendPath(target), "original backend")
	writeTest(t, target+".eden-installation.json", "pending")
	if OriginalIntact(target, backup) {
		t.Fatal("unresolved receipt considered intact")
	}
	if e := os.Remove(target + ".eden-installation.json"); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(target, "Eden.exe")); e != nil {
		t.Fatal(e)
	}
	if OriginalIntact(target, backup) {
		t.Fatal("missing launcher considered intact")
	}
	if readTest(t, filepath.Join(target, "my-project", "work.txt")) != "latest user data" {
		t.Fatal("classification changed personal data")
	}
}

func TestIncompleteInstallationReportsPreservedDirectories(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if e := os.Mkdir(target, 0700); e != nil {
		t.Fatal(e)
	}
	if e := EnsureInstallationComplete(target); e != nil {
		t.Fatal(e)
	}
	stage, backup := filepath.Join(root, "stage"), filepath.Join(root, "backup")
	b, e := json.Marshal(map[string]string{"stagePath": stage, "backupPath": backup})
	if e != nil {
		t.Fatal(e)
	}
	receipt := target + ".eden-installation.json"
	writeTest(t, receipt, string(b))
	e = EnsureInstallationComplete(target)
	if e == nil || !strings.Contains(e.Error(), stage) || !strings.Contains(e.Error(), backup) {
		t.Fatal("omitted recovery paths", e)
	}
	if _, e = os.Stat(receipt); e != nil {
		t.Fatal("removed recovery receipt")
	}
	if e = EnsureInstallationComplete(filepath.Join(root, "missing")); e == nil {
		t.Fatal("missing install accepted")
	}
}

func TestReadLockPreventsReplacement(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "archive.exe")
	writeTest(t, name, "archive")
	f, e := installfiles.OpenReadLocked(name)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = os.Rename(name, filepath.Join(root, "renamed.exe")); e == nil {
		t.Fatal("verified file replaced under read lock")
	}
	if e = os.WriteFile(name, []byte("changed"), 0600); e == nil {
		t.Fatal("verified file changed under read lock")
	}
	if readTest(t, name) != "archive" {
		t.Fatal("archive changed")
	}
}

func TestParentIdentityChecks(t *testing.T) {
	image, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ticks, e := CurrentStartTicks()
	if e != nil {
		t.Fatal(e)
	}
	parent, e := openParent(os.Getpid(), ticks, image)
	if e != nil {
		t.Fatal(e)
	}
	parent.Close()
	for _, tc := range []struct {
		ticks int64
		image string
	}{{ticks + 1, image}, {ticks, filepath.Join(t.TempDir(), "other.exe")}} {
		if p, e := openParent(os.Getpid(), tc.ticks, tc.image); e == nil {
			p.Close()
			t.Fatal("parent identity mismatch accepted")
		}
	}
	if p, e := openParent(-1, ticks, image); e == nil {
		p.Close()
		t.Fatal("invalid PID accepted")
	}
}

func TestUnsignedProgramRejected(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "unsigned.exe")
	writeTest(t, name, "not a signed program")
	if _, e := Publisher(name); e == nil {
		t.Fatal("unsigned content trusted")
	}
	if _, _, e := ProductVersion(name); e == nil {
		t.Fatal("invalid version resource accepted")
	}
}

func TestMarkReadyOnlyAfterExplicitCall(t *testing.T) {
	b := &fakeBackend{update: func(context.Context, bool, func(appupdate.State)) error { return nil }}
	u := testUpdater(t, b, nil)
	confirmation := newConfirmation()
	t.Setenv("EDEN_UPDATE_CONFIRMATION", confirmation)
	marker := filepath.Join(u.config.Root, "launch-"+confirmation+".json")
	if _, e := os.Lstat(marker); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("acknowledged before workbench ready")
	}
	if e := u.MarkReady(); e != nil {
		t.Fatal(e)
	}
	var result struct {
		Version string `json:"version"`
	}
	if e := readSmallJSON(marker, &result, 16384); e != nil {
		t.Fatal(e)
	}
	if result.Version != "0.1.0" || u.Status().Phase != "current" {
		t.Fatal(result, u.Status())
	}
	if os.Getenv("EDEN_UPDATE_CONFIRMATION") != "" {
		t.Fatal("confirmation environment leaked")
	}
}

func TestCleanupFailureDoesNotAffectHealthyApp(t *testing.T) {
	root := t.TempDir()
	backup, download := filepath.Join(root, "backup"), filepath.Join(root, "download")
	locked := filepath.Join(backup, "old.dll")
	writeTest(t, locked, "old")
	writeTest(t, filepath.Join(download, "archive.exe"), "download")
	app := filepath.Join(root, "installed", "Eden.exe")
	writeTest(t, app, "healthy")
	f, e := installfiles.OpenReadLocked(locked)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	cleanupDirectory(backup)
	cleanupDirectory(download)
	if readTest(t, locked) != "old" || readTest(t, app) != "healthy" {
		t.Fatal("cleanup changed locked file or healthy application")
	}
	if _, e = os.Stat(download); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("one cleanup failure blocked independent cleanup")
	}
}

// Child modes perform only workspace fixture writes and waits. They never
// launch the installer, a controller, or any host-system integration command.
func TestProcessLifecycleFixture(t *testing.T) {
	mode := os.Getenv("EDEN_UPDATE_TEST_PROCESS")
	if mode == "" {
		return
	}
	marker := os.Getenv("EDEN_UPDATE_TEST_MARKER")
	if !filepath.IsAbs(marker) || !strings.HasPrefix(strings.ToLower(filepath.Clean(marker)), strings.ToLower(filepath.Clean(os.Getenv("EDEN_WORKSPACE_ROOT")))+string(filepath.Separator)) {
		os.Exit(92)
	}
	if e := os.WriteFile(marker, []byte("started"), 0600); e != nil {
		os.Exit(93)
	}
	switch mode {
	case "success":
		os.Exit(0)
	case "failure":
		os.Exit(7)
	case "wait":
		time.Sleep(30 * time.Second)
		os.Exit(94)
	case "spawn-child":
		childMarker, pidRecord := os.Getenv("EDEN_UPDATE_TEST_CHILD_MARKER"), os.Getenv("EDEN_UPDATE_TEST_CHILD_PID")
		for _, name := range []string{childMarker, pidRecord} {
			if !filepath.IsAbs(name) || !strings.HasPrefix(strings.ToLower(filepath.Clean(name)), strings.ToLower(filepath.Clean(os.Getenv("EDEN_WORKSPACE_ROOT")))+string(filepath.Separator)) {
				os.Exit(96)
			}
		}
		child := exec.Command(os.Args[0], "-test.run=^TestProcessLifecycleFixture$")
		hide(child)
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, "EDEN_UPDATE_TEST_PROCESS") || strings.EqualFold(key, "EDEN_UPDATE_TEST_MARKER") {
				continue
			}
			child.Env = append(child.Env, entry)
		}
		child.Env = append(child.Env, "EDEN_UPDATE_TEST_PROCESS=wait", "EDEN_UPDATE_TEST_MARKER="+childMarker)
		if e := child.Start(); e != nil {
			os.Exit(97)
		}
		if e := os.WriteFile(pidRecord, []byte(strconv.Itoa(child.Process.Pid)), 0600); e != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
			os.Exit(98)
		}
		_ = child.Process.Release()
		time.Sleep(30 * time.Second)
		os.Exit(99)
	default:
		os.Exit(95)
	}
}

func TestOwnedProcessSuccessFailureCancellationAndTimeout(t *testing.T) {
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"success", "failure", "cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "child-started")
			childMode := mode
			if mode == "cancel" || mode == "timeout" {
				childMode = "wait"
			}
			cmd := exec.Command(executable, "-test.run=^TestProcessLifecycleFixture$")
			hide(cmd)
			cmd.Env = append(os.Environ(), "EDEN_UPDATE_TEST_PROCESS="+childMode, "EDEN_UPDATE_TEST_MARKER="+marker)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if mode == "timeout" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
			}
			if mode == "cancel" {
				go func() {
					deadline := time.Now().Add(3 * time.Second)
					for time.Now().Before(deadline) {
						if _, e := os.Stat(marker); e == nil {
							cancel()
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
					cancel()
				}()
			}
			e := runProcess(ctx, cmd)
			if mode == "success" && e != nil {
				t.Fatal(e)
			}
			if mode == "failure" {
				var exit *exec.ExitError
				if !errors.As(e, &exit) || exit.ExitCode() != 7 {
					t.Fatal("lost child exit status", e)
				}
			}
			if mode == "cancel" && !errors.Is(e, context.Canceled) {
				t.Fatal("lost cancellation", e)
			}
			if mode == "timeout" && !errors.Is(e, context.DeadlineExceeded) {
				t.Fatal("lost deadline", e)
			}
			if _, e = os.Stat(marker); e != nil {
				t.Fatal("child never reached real process entry", e)
			}
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				t.Fatal("owned process was not reaped")
			}
		})
	}
}

func TestNSISCommandLine(t *testing.T) {
	root := t.TempDir()
	archive, target, diagnostics := filepath.Join(root, "archive with spaces.exe"), filepath.Join(root, "installed application"), filepath.Join(root, "diagnostic report.json")
	cmd := installerCommand(archive, target, diagnostics)
	if cmd.Path != archive || cmd.Dir != filepath.Dir(archive) || !strings.HasSuffix(cmd.SysProcAttr.CmdLine, " /D="+target) || strings.HasSuffix(cmd.SysProcAttr.CmdLine, `"`) {
		t.Fatal("NSIS final unquoted destination changed", cmd.SysProcAttr.CmdLine)
	}
	if !strings.Contains(cmd.SysProcAttr.CmdLine, "/S /UPDATE") || !strings.Contains(cmd.SysProcAttr.CmdLine, "/DIAGNOSTICS=") {
		t.Fatal(cmd.SysProcAttr.CmdLine)
	}
}

func TestPinnedParentWaitCancellationAndActualExit(t *testing.T) {
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(t.TempDir(), "parent-started")
	cmd := exec.Command(executable, "-test.run=^TestProcessLifecycleFixture$")
	hide(cmd)
	cmd.Env = append(os.Environ(), "EDEN_UPDATE_TEST_PROCESS=wait", "EDEN_UPDATE_TEST_MARKER="+marker)
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(cmd.Process.Pid))
	if e != nil {
		t.Fatal(e)
	}
	ticks, e := processTicks(h)
	windows.CloseHandle(h)
	if e != nil {
		t.Fatal(e)
	}
	parent, e := openParent(cmd.Process.Pid, ticks, executable)
	if e != nil {
		t.Fatal(e)
	}
	defer parent.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if e = parent.Wait(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("pinned live parent did not honor wait deadline", e)
	}
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	ctx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if e = parent.Wait(ctx); e != nil {
		t.Fatal("pinned parent exit not observed", e)
	}
}

func TestCanceledOwnedJobAlsoStopsDescendants(t *testing.T) {
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	marker, childMarker, pidRecord := filepath.Join(root, "parent-started"), filepath.Join(root, "child-started"), filepath.Join(root, "child-pid")
	cmd := exec.Command(executable, "-test.run=^TestProcessLifecycleFixture$")
	hide(cmd)
	cmd.Env = append(os.Environ(), "EDEN_UPDATE_TEST_PROCESS=spawn-child", "EDEN_UPDATE_TEST_MARKER="+marker, "EDEN_UPDATE_TEST_CHILD_MARKER="+childMarker, "EDEN_UPDATE_TEST_CHILD_PID="+pidRecord)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runProcess(ctx, cmd) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, e = os.Stat(childMarker); e == nil {
			if info, e := os.Stat(pidRecord); e == nil && info.Size() > 0 {
				break
			}
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("owned descendant did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	pid, e := strconv.Atoi(readTest(t, pidRecord))
	if e != nil {
		cancel()
		<-done
		t.Fatal(e)
	}
	child, e := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, uint32(pid))
	if e != nil {
		cancel()
		<-done
		t.Fatal(e)
	}
	defer windows.CloseHandle(child)
	defer windows.TerminateProcess(child, 1)
	cancel()
	if e = <-done; !errors.Is(e, context.Canceled) {
		t.Fatal("lost process tree cancellation", e)
	}
	if status, e := windows.WaitForSingleObject(child, 1000); e != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("owned descendant survived canceled job", status, e)
	}
}

func TestResultAtomicReplacementDoesNotMutateHardLink(t *testing.T) {
	root := t.TempDir()
	original, result := filepath.Join(root, "personal-original"), filepath.Join(root, "last-result.json")
	writeTest(t, original, "personal bytes")
	if e := os.Link(original, result); e != nil {
		t.Fatal(e)
	}
	if e := writeResultFile(result, map[string]bool{"success": true}); e != nil {
		t.Fatal(e)
	}
	if readTest(t, original) != "personal bytes" {
		t.Fatal("result update followed a hard link and changed personal bytes")
	}
	var saved map[string]bool
	if e := readSmallJSON(result, &saved, 16384); e != nil || !saved["success"] {
		t.Fatal(saved, e)
	}
}

func TestSnapshotCanceledBeforeCreationAndCollisionOwnership(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeTest(t, filepath.Join(source, "program.txt"), "program")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	owned := false
	if e := copyTree(ctx, source, target, &owned); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if owned {
		t.Fatal("canceled copy claimed directory")
	}
	if _, e := os.Stat(target); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("canceled copy created output")
	}
	writeTest(t, filepath.Join(target, "personal.txt"), "personal")
	if e := copyTree(context.Background(), source, target, &owned); e == nil || owned {
		t.Fatal("colliding snapshot claimed existing directory", e)
	}
	if readTest(t, filepath.Join(target, "personal.txt")) != "personal" {
		t.Fatal("snapshot changed existing data")
	}
}

func TestMarkReadyRejectsInvalidConfirmationWithoutWriting(t *testing.T) {
	b := &fakeBackend{}
	u := testUpdater(t, b, nil)
	for _, token := range []string{"", "../outside", "12345678-1234-1234-1234-123456789abz"} {
		t.Setenv("EDEN_UPDATE_CONFIRMATION", token)
		if e := u.MarkReady(); e != nil {
			t.Fatal(e)
		}
		if _, e := os.Lstat(u.config.Root); !errors.Is(e, os.ErrNotExist) {
			t.Fatal("invalid confirmation created update state")
		}
	}
}

func sparseFixture(t *testing.T, name string, size int64) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(name), 0700); e != nil {
		t.Fatal(e)
	}
	f, e := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	var returned uint32
	if e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &returned, nil); e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(size); e != nil {
		t.Fatal(e)
	}
}

func TestArchiveVerificationFailureBoundariesReleaseLocks(t *testing.T) {
	content := "unsigned archive fixture"
	h := sha256.Sum256([]byte(content))
	validDigest := hex.EncodeToString(h[:])
	for _, tc := range []struct{ name, content, digest, want string }{{"empty", "", validDigest, "大小无效"}, {"malformed-digest", content, "not-hex", "摘要无效"}, {"changed-bytes", content, strings.Repeat("0", 64), "下载后发生变化"}, {"untrusted-signature", content, validDigest, "Windows 无法验证安装包签名"}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "download-fixture", "Eden-setup.exe")
			writeTest(t, archive, tc.content)
			f, e := VerifyArchiveLocked(root, archive, tc.digest, "0.2.0", "expected publisher")
			if f != nil {
				f.Close()
				t.Fatal("returned trusted handle on failure")
			}
			if e == nil || !strings.Contains(e.Error(), tc.want) {
				t.Fatal("wrong verification branch", e)
			}
			if e = os.Rename(archive, archive+".retained"); e != nil {
				t.Fatal("verification failure leaked read lock", e)
			}
		})
	}
	t.Run("oversized-sparse-archive", func(t *testing.T) {
		root := t.TempDir()
		archive := filepath.Join(root, "download-fixture", "Eden-setup.exe")
		sparseFixture(t, archive, (512<<20)+1)
		if f, e := VerifyArchiveLocked(root, archive, validDigest, "0.2.0", "publisher"); e == nil || !strings.Contains(e.Error(), "大小无效") {
			if f != nil {
				f.Close()
			}
			t.Fatal("archive size boundary not enforced before hashing", e)
		}
	})
}

func TestBackupSizeBudgetRejectsSparseOversizeBeforeCopy(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "backup")
	name := filepath.Join(source, "oversized-program.bin")
	sparseFixture(t, name, (4<<30)+1)
	if e := CopyTree(context.Background(), source, target); e == nil || !strings.Contains(e.Error(), "备份大小") {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(target, "oversized-program.bin")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("copied oversized payload before checking budget")
	}
}

func TestRecoveryRelaunchFailureRetainsVerifiedBackup(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "previous-application")
	original := backup
	writeTest(t, filepath.Join(backup, "Eden.exe"), "verified previous application")
	launchFailure := errors.New("controlled process creation failure")
	if e := reopenRecovered(&backup, func() error { return launchFailure }); !errors.Is(e, launchFailure) {
		t.Fatal(e)
	}
	if backup != original || readTest(t, filepath.Join(original, "Eden.exe")) != "verified previous application" {
		t.Fatal("failed relaunch discarded the recovery snapshot")
	}
	if e := reopenRecovered(&backup, func() error {
		if _, e := os.Stat(filepath.Join(original, "Eden.exe")); e != nil {
			t.Fatal("snapshot removed before process creation", e)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if backup != "" {
		t.Fatal("successful relaunch retained stale backup state")
	}
	if _, e := os.Stat(original); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("successful relaunch did not clean owned snapshot")
	}
}
