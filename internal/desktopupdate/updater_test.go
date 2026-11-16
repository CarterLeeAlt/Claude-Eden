package desktopupdate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/appupdate"
)

func TestVersionOrdering(t *testing.T) {
	ordered := []string{"0.1.0-alpha", "0.1.0-alpha.0", "0.1.0-alpha.1", "0.1.0-alpha.9", "0.1.0-alpha.10", "0.1.0-alpha.beta", "0.1.0-beta", "0.1.0", "0.1.1", "0.2.0", "1.0.0", "999999999999999999999999999999999.0.0"}
	for i, a := range ordered {
		for j, b := range ordered {
			if IsNewer(a, b) != (i > j) {
				t.Errorf("IsNewer(%q,%q) incorrect", a, b)
			}
		}
	}
	for _, invalid := range []string{"", "development", "v0.2.0", "0.2", "00.2.0", "0.2.0+metadata", "0.2.0-alpha.01", "0.2.0-", "0.2.0-alpha..1", "0.2.0\n"} {
		if IsNewer(invalid, "0.1.0") || IsNewer("0.2.0", invalid) {
			t.Errorf("accepted invalid version %q", invalid)
		}
	}
}

func TestDownloadLocationPolicy(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "download-abc", "Eden-setup.exe")
	if got, e := DownloadPath(good, root); e != nil || got != good {
		t.Fatal(got, e)
	}
	for _, bad := range []string{"", "Eden-setup.exe", filepath.Join(root+"-other", "download-abc", "Eden-setup.exe"), filepath.Join(root, "verified-abc", "Eden-setup.exe"), filepath.Join(root, "download-abc", "nested", "Eden-setup.exe"), filepath.Join(root, "download-abc", "Eden.exe"), filepath.Join(root, "download-abc", "eden-setup.exe")} {
		if _, e := DownloadPath(bad, root); e == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

type fakeBackend struct {
	mu     sync.Mutex
	calls  int
	update func(context.Context, bool, func(appupdate.State)) error
}

func (b *fakeBackend) Update(ctx context.Context, check bool, emit func(appupdate.State)) error {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	return b.update(ctx, check, emit)
}
func (b *fakeBackend) PrepareUpdate(ctx context.Context, stop bool) error {
	return errors.New("unexpected installation preparation")
}
func (b *fakeBackend) count() int { b.mu.Lock(); defer b.mu.Unlock(); return b.calls }
func testUpdater(t *testing.T, b Backend, changed func(State)) *Updater {
	t.Helper()
	root := t.TempDir()
	u, e := New(Config{Root: filepath.Join(root, "updates"), Target: filepath.Join(root, "installed"), Executable: filepath.Join(root, "installed", "Eden.exe"), Version: "0.1.0", ConfigPath: filepath.Join(root, "config.json"), Backend: b, Changed: changed})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(u.Close)
	return u
}
func waitFinished(t *testing.T, u *Updater) {
	t.Helper()
	u.mu.Lock()
	done := u.done
	u.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("updater did not terminate")
		}
	}
}

func TestUpdaterCooldownAndTypedState(t *testing.T) {
	retry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	b := &fakeBackend{update: func(ctx context.Context, check bool, emit func(appupdate.State)) error {
		emit(appupdate.State{Phase: "failed", Message: "limited", Retryable: true, RetryAt: retry, ArchivePath: "private", SHA256: "private"})
		return errors.New("limited")
	}}
	u := testUpdater(t, b, nil)
	if _, e := u.Handle("check"); e != nil {
		t.Fatal(e)
	}
	waitFinished(t, u)
	s := u.Status()
	if s.RetryAt != retry || s.Phase != "failed" {
		t.Fatal(s)
	}
	for _, op := range []string{"check", "install"} {
		if _, e := u.Handle(op); e != nil {
			t.Fatal(e)
		}
	}
	if b.count() != 1 {
		t.Fatal("server retry deadline bypassed")
	}
	encoded, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "archivePath") || strings.Contains(string(encoded), "sha256") {
		t.Fatal("private update fields leaked to UI")
	}
	if _, e = u.Handle("invalid"); e == nil {
		t.Fatal("unknown operation accepted")
	}
}

func TestUpdaterCancelCloseAndDuplicateRequests(t *testing.T) {
	started := make(chan struct{})
	b := &fakeBackend{update: func(ctx context.Context, check bool, emit func(appupdate.State)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	u := testUpdater(t, b, nil)
	_, _ = u.Handle("check")
	<-started
	for _, op := range []string{"check", "install", "status"} {
		_, _ = u.Handle(op)
	}
	if b.count() != 1 {
		t.Fatal("concurrent update started")
	}
	_, _ = u.Handle("cancel")
	waitFinished(t, u)
	if s := u.Status(); s.Phase != "failed" || !s.Retryable {
		t.Fatal(s)
	}
	u.Close()
	if _, e := u.Handle("check"); e == nil {
		t.Fatal("closed updater accepted request")
	}
}

func TestUpdaterEmitsCheckStates(t *testing.T) {
	var mu sync.Mutex
	states := []State{}
	b := &fakeBackend{update: func(ctx context.Context, check bool, emit func(appupdate.State)) error {
		if !check {
			t.Error("check became download")
		}
		emit(appupdate.State{Phase: "available", LatestVersion: "0.2.0", Message: "available"})
		return nil
	}}
	u := testUpdater(t, b, func(s State) { mu.Lock(); states = append(states, s); mu.Unlock() })
	_, _ = u.Handle("check")
	waitFinished(t, u)
	mu.Lock()
	defer mu.Unlock()
	if len(states) != 2 || states[0].Phase != "checking" || states[1].Phase != "available" || states[1].CurrentVersion != "0.1.0" {
		t.Fatal(states)
	}
}

func TestBoundedProtocol(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		fail        bool
	}{{"READY\n", "READY", false}, {"READY\r\n", "READY", false}, {"", "", true}, {"READY", "", true}, {strings.Repeat("x", 20) + "\n", "", true}} {
		got, e := readLine(context.Background(), strings.NewReader(tc.input), 16)
		if (e != nil) != tc.fail || got != tc.want {
			t.Fatalf("got %q %v", got, e)
		}
	}
	reader, writer := io.Pipe()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := readLine(ctx, reader, 16); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	buffered := bufio.NewReader(strings.NewReader("plan\nCOMMIT\n"))
	for _, want := range []string{"plan", "COMMIT"} {
		if got, e := readLine(context.Background(), buffered, 16); e != nil || got != want {
			t.Fatalf("buffered protocol lost next line: %q %v", got, e)
		}
	}
	for _, ending := range []string{"\n", "\r\n"} {
		if got, e := readLine(context.Background(), strings.NewReader(strings.Repeat("x", 16)+ending), 16); e != nil || len(got) != 16 {
			t.Fatal("rejected exact line-size boundary", got, e)
		}
	}
}

func TestNewRequiresConfiguration(t *testing.T) {
	if _, e := New(Config{}); e == nil {
		t.Fatal("accepted incomplete configuration")
	}
}

func TestPrepareDownloadedRejectsVersionBeforeNativeVerification(t *testing.T) {
	u := testUpdater(t, &fakeBackend{}, nil)
	for _, version := range []string{"0.1.0", "0.0.9", "development"} {
		err := u.prepareDownloaded(context.Background(), appupdate.State{LatestVersion: version}, filepath.Join(u.config.Root, "download-test", "Eden-setup.exe"))
		if err == nil || !strings.Contains(err.Error(), "不高于当前版本") {
			t.Fatal(version, err)
		}
		if u.prepared != nil || u.Status().Phase != "idle" {
			t.Fatal("invalid version changed preparation state")
		}
	}
}

func TestProfileEnvironment(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "selected profile", "config.json")
	env := profileEnvironment(config, "confirmation")
	count := 0
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(key, "EDEN_CONFIG") {
			count++
			if value != config {
				t.Fatal("profile not preserved")
			}
		}
	}
	if count != 1 {
		t.Fatal("selected profile duplicated or lost")
	}
	_ = os.Getenv("EDEN_CONFIG")
}

type commitWriter struct {
	entered, release chan struct{}
	n                int
	err              error
}

func (w *commitWriter) Write(p []byte) (int, error) {
	if w.entered != nil {
		close(w.entered)
		<-w.release
	}
	return w.n, w.err
}

func TestCommitCancellationOrdering(t *testing.T) {
	t.Run("cancel-before-write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		u := &Updater{cancel: cancel}
		if _, e := u.Handle("cancel"); e != nil {
			t.Fatal(e)
		}
		var output bytes.Buffer
		if e := u.commit(ctx, &output); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		if output.Len() != 0 || u.committed {
			t.Fatal("revoked update sent COMMIT")
		}
	})
	t.Run("commit-before-cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		u := &Updater{cancel: cancel}
		writer := &commitWriter{entered: make(chan struct{}), release: make(chan struct{}), n: len("COMMIT\n")}
		written := make(chan error, 1)
		go func() { written <- u.commit(ctx, writer) }()
		<-writer.entered
		canceled := make(chan error, 1)
		go func() { _, e := u.Handle("cancel"); canceled <- e }()
		close(writer.release)
		if e := <-written; e != nil {
			t.Fatal(e)
		}
		if e := <-canceled; e != nil {
			t.Fatal(e)
		}
		if !u.committed || ctx.Err() != nil {
			t.Fatal("accepted COMMIT was later revoked")
		}
	})
	for _, tc := range []struct {
		name      string
		n         int
		err       error
		committed bool
	}{{"short-write", 2, nil, false}, {"failed-write", 0, io.ErrClosedPipe, false}, {"complete-with-close-error", len("COMMIT\n"), io.ErrClosedPipe, true}} {
		t.Run(tc.name, func(t *testing.T) {
			u := &Updater{}
			e := u.commit(context.Background(), &commitWriter{n: tc.n, err: tc.err})
			if u.committed != tc.committed || (e == nil) != tc.committed {
				t.Fatal("incorrect ownership transfer", e, u.committed)
			}
		})
	}
}

func TestMissingQuitCallbackCannotCommitInstallation(t *testing.T) {
	u := testUpdater(t, &fakeBackend{}, nil)
	u.install(context.Background())
	if s := u.Status(); s.Phase != "failed" || s.Retryable || u.committed {
		t.Fatal("installation accepted without a real application-exit callback", s)
	}
	if _, e := os.Lstat(u.config.Root); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("missing callback caused installation side effects")
	}
}
