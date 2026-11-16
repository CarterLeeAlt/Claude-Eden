//go:build unix

package s

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestMOVE_CrossDevice(t *testing.T) {
	volume := os.Getenv("WAILS_TEST_STORAGE_VOLUME")
	if volume == "" {
		t.Skip("set WAILS_TEST_STORAGE_VOLUME to an explicitly authorized cross-device test directory")
	}
	if !filepath.IsAbs(volume) || filepath.Clean(volume) == string(filepath.Separator) {
		t.Fatal("WAILS_TEST_STORAGE_VOLUME must be an absolute dedicated directory")
	}
	for ancestor := filepath.Clean(volume); ; ancestor = filepath.Dir(ancestor) {
		info, err := os.Lstat(ancestor)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("invalid cross-device test ancestor %q: %v", ancestor, err)
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	srcDir := t.TempDir()
	dstDir, err := os.MkdirTemp(volume, "wails-move-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dstDir); err != nil {
			t.Error(err)
		}
	})
	var a, b syscall.Stat_t
	if syscall.Stat(srcDir, &a) != nil || syscall.Stat(dstDir, &b) != nil || a.Dev == b.Dev {
		t.Fatal("the explicitly configured test directory must be on another filesystem")
	}

	src := filepath.Join(srcDir, "app.AppImage")
	if err := os.WriteFile(src, []byte("elf"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the fixture executable independently of the test runner's umask.
	if err := os.Chmod(src, 0o755); err != nil {
		t.Fatal(err)
	}
	// A symlink at the target is replaced, as rename would, not written through.
	linked := filepath.Join(dstDir, "linked")
	if err := os.WriteFile(linked, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linked, filepath.Join(dstDir, "app.AppImage")); err != nil {
		t.Fatal(err)
	}
	MOVE(src, dstDir)

	if b, err := os.ReadFile(linked); err != nil || string(b) != "old" {
		t.Errorf("the symlink's file was written through: %q, %v", b, err)
	}

	got, err := os.Stat(filepath.Join(dstDir, "app.AppImage"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", got.Mode().Perm())
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source still present: %v", err)
	}
}
