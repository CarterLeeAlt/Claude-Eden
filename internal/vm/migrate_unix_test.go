//go:build unix

package vm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func device(p string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(p, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}

// To another file system: copied, then removed (rename cannot).
func TestMoveOldAcrossFS(t *testing.T) {
	other := os.Getenv("AGENT_VM_TEST_OTHER_FS")
	if other == "" {
		t.Skip("cross-filesystem integration test: explicitly set AGENT_VM_TEST_OTHER_FS to a writable directory on another filesystem")
	}
	info, err := os.Stat(other)
	if err != nil {
		t.Fatalf("AGENT_VM_TEST_OTHER_FS=%q: %v", other, err)
	}
	if !info.IsDir() {
		t.Fatalf("AGENT_VM_TEST_OTHER_FS=%q is not a directory", other)
	}
	// TempDir calls share this test's private parent. Validate the configured
	// destination before newTestLima can skip because guest assets are absent.
	from, err := device(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	to, err := device(other)
	if err != nil {
		t.Fatalf("AGENT_VM_TEST_OTHER_FS=%q: %v", other, err)
	}
	if to == from {
		t.Fatalf("AGENT_VM_TEST_OTHER_FS=%q is on the source filesystem", other)
	}
	tmp, err := os.MkdirTemp(other, "avm")
	if err != nil {
		t.Fatalf("create cross-filesystem fixture in %q: %v", other, err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	l := newTestLima(t)
	old := os.Getenv("LIMA_HOME")
	if actual, err := device(old); err != nil || actual != from {
		t.Fatalf("source filesystem changed: device=%d, expected=%d, error=%v", actual, from, err)
	}
	none := []Mount{}
	if err := l.Create(ctx, "agent-vm-p-12345678", "template:debian-13", Settings{CPUs: 1, Mounts: &none}, io.Discard); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(old, "agent-vm-p-12345678")
	if err := os.WriteFile(filepath.Join(dir, "x.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("lima.yaml", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	now := filepath.Join(tmp, "lima")
	if err := MoveOld(ctx, old, now, "agent-vm-p-12345678", "p-12345678"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("left in the old home")
	}
	moved := filepath.Join(now, "p-12345678")
	if _, err := os.Stat(filepath.Join(moved, "x.tmp")); err == nil {
		t.Error("x.tmp was moved")
	}
	if l, err := os.Readlink(filepath.Join(moved, "link")); err != nil || l != "lima.yaml" {
		t.Errorf("link: %q %v", l, err)
	}
	l2, err := NewLima(now, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if inst, err := l2.Get(ctx, "p-12345678"); err != nil || inst.CPUs != 1 || inst.Status != Stopped {
		t.Fatalf("in the new home: %+v %v", inst, err)
	}
}
