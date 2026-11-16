//go:build !windows

package workbench

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

type stoppedShareMarker struct{ path string }

func (m stoppedShareMarker) Read([]byte) (int, error) { return 0, io.EOF }
func (m stoppedShareMarker) Close() error             { return os.WriteFile(m.path, []byte("closed"), 0600) }

func TestInternalEnvironmentStopKeepsProxy(t *testing.T) {
	s := transferMountServer(t, false, "current")
	bin := t.TempDir()
	args := filepath.Join(bin, "stop-args")
	t.Setenv("EDEN_TEST_STOP_ARGS", args)
	s.exe = filepath.Join(bin, "eden")
	if err := os.WriteFile(s.exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$EDEN_TEST_STOP_ARGS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.stopEnvironment(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(args)
	if err != nil || string(got) != "stop\n--keep-proxy\n" {
		t.Fatalf("internal environment stop must preserve the active proxy: %q, %v", got, err)
	}
}

func TestEnvironmentStopKeepsSharesUntilGuestExit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved", true: "stop-failed"}[fail], func(t *testing.T) {
			s := transferMountServer(t, false, "current")
			root := t.TempDir()
			closed, saved := filepath.Join(root, "share-closed"), filepath.Join(root, "guest-saved")
			t.Setenv("EDEN_TEST_SHARE_CLOSED", closed)
			t.Setenv("EDEN_TEST_GUEST_SAVED", saved)
			t.Setenv("EDEN_TEST_STOP_FAILURE", map[bool]string{false: "0", true: "1"}[fail])
			reader, writer := io.Pipe()
			defer reader.Close()
			done := make(chan struct{})
			close(done)
			share := &projectShare{transport: projectTransport{stoppedShareMarker{closed}, writer}, done: done}
			s.projectShares = map[string]*projectShare{root: share}
			defer s.closeProjectShares()
			s.exe = filepath.Join(root, "eden")
			if err := os.WriteFile(s.exe, []byte(`#!/bin/sh
test ! -e "$EDEN_TEST_SHARE_CLOSED" || { echo 'shared transport closed before guest exit' >&2; exit 7; }
test "$EDEN_TEST_STOP_FAILURE" = 0 || exit 8
printf 'saved during shutdown' > "$EDEN_TEST_GUEST_SAVED"
`), 0700); err != nil {
				t.Fatal(err)
			}
			err := s.stopEnvironment()
			if fail {
				if err == nil || s.projectShares[root] != share {
					t.Fatalf("failed stop discarded the live share: %v", err)
				}
				if _, err := os.Stat(closed); !os.IsNotExist(err) {
					t.Fatalf("failed stop closed the share: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(saved); err != nil || string(data) != "saved during shutdown" {
					t.Fatalf("guest shutdown could not save: %q %v", data, err)
				}
				if _, err := os.Stat(closed); err != nil || len(s.projectShares) != 0 {
					t.Fatalf("stopped guest retained its share: %v", err)
				}
			}
		})
	}
}
