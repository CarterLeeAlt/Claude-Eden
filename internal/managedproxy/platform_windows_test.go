package managedproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
