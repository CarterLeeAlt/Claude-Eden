package managedproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsControlSocketFitsWorkspaceTempAndRejectsLongPaths(t *testing.T) {
	profile := t.TempDir()
	temporary := os.TempDir()
	s, err := Load(profile)
	if err != nil {
		t.Fatal(err)
	}
	s.Nodes = []Node{{ID: Token()[:24], Name: "test", Type: "http", Server: "127.0.0.1", Port: 1080}}
	s.Selected = s.Nodes[0].ID
	socket := Socket(profile)
	if filepath.Dir(filepath.Dir(socket)) != filepath.Clean(temporary) || Socket(profile) != socket || Socket(filepath.Join(profile, "other")) == socket {
		t.Fatal("socket no longer stays in a profile-specific workspace temp directory")
	}
	if len(socket) >= 108 {
		t.Fatalf("workspace socket exceeds AF_UNIX limit: %d bytes", len(socket))
	}
	if _, err := s.Runtime(profile); err != nil {
		t.Fatal(err)
	}
	// This is a string-only invalid configuration, not a real write target.
	longTemp := filepath.Join(temporary, strings.Repeat("long", 40))
	t.Setenv("TMP", longTemp)
	t.Setenv("TEMP", longTemp)
	if _, err := s.Runtime(profile); err == nil || !strings.Contains(err.Error(), "107") {
		t.Fatalf("long socket was not rejected before writes: %v", err)
	}
	if _, err := os.Stat(longTemp); !os.IsNotExist(err) {
		t.Fatalf("invalid temp path was created or inaccessible: %v", err)
	}
}

// A PID reused after a reboot must neither be terminated nor block startup.
func TestStopWindowsCoreForgetsReusedPID(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(filepath.Dir(exe), "mihomo.exe")
	f, err := os.OpenFile(core, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err == nil {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(core) })
	} else if !os.IsExist(err) {
		t.Fatal(err)
	}
	for name, pid := range map[string]uint32{"same user": uint32(os.Getpid()), "system": 4} {
		t.Run(name, func(t *testing.T) {
			profile := t.TempDir()
			b, _ := json.Marshal(windowsCoreRecord{PID: pid, Executable: core, Created: 1})
			if err := Write(windowsCoreRecordPath(profile), b); err != nil {
				t.Fatal(err)
			}
			if err := stopWindowsCore(profile); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(windowsCoreRecordPath(profile)); !os.IsNotExist(err) {
				t.Fatal("stale core record was kept")
			}
		})
	}
}
