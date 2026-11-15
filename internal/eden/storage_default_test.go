package eden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsInitialDataDirectoryAndApplicationBoundary(t *testing.T) {
	profile, root := newStorageFixture(t)
	app := filepath.Join(root, "Eden")
	backend := filepath.Join(app, "resources", "eden.exe")
	writeStorageFixture(t, backend, []byte("backend"))
	writeStorageFixture(t, filepath.Join(app, "Eden.exe"), []byte("launcher"))
	if got := initialStorageDirectory(backend, "windows"); got != "" {
		t.Fatalf("unmarked build default=%s", got)
	}
	writeStorageFixture(t, filepath.Join(app, "Eden.install-id"), []byte("io.github.carterleealt.claude-eden"))
	data := filepath.Join(app, "EdenData")
	if got := initialStorageDirectory(backend, "windows"); got != data {
		t.Fatalf("default=%s want=%s", got, data)
	}
	for _, platform := range []string{"linux"} {
		if got := initialStorageDirectory(backend, platform); got != "" {
			t.Fatalf("%s default changed to %s", platform, got)
		}
	}
	if err := validateStorageTarget(profile, data, app, "windows"); err != nil {
		t.Fatalf("reserved Windows data folder rejected: %v", err)
	}
	for _, target := range []string{app, filepath.Join(app, "resources", "data"), filepath.Join(app, "OtherData")} {
		if err := validateStorageTarget(profile, target, app, "windows"); err == nil {
			t.Fatalf("program boundary opened for %s", target)
		}
	}
	if err := validateStorageTarget(profile, data, app, "linux"); err == nil {
		t.Fatal("server application directory allowed")
	}
	if err := os.Remove(filepath.Join(app, "Eden.install-id")); err != nil {
		t.Fatal(err)
	}
	if err := validateStorageTarget(profile, data, app, "windows"); err == nil {
		t.Fatal("unmarked installation allowed data inside app")
	}
}
