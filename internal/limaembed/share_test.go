package limaembed

import (
	"os"
	"testing"
	"testing/fstest"
)

func TestGuestAgentMatchesGuestAndCurrentBuild(t *testing.T) {
	linuxArch := guestArch()
	const extraFile = "assets/lima-guestagent.Linux-test-arch.gz"
	files := fstest.MapFS{
		"assets/lima-guestagent.Linux-" + linuxArch + ".gz": {Data: []byte("linux")},
		extraFile: {Data: []byte("extra")},
	}
	dir := t.TempDir()
	share, err := writeShare(dir, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, guest := range []struct{ os, arch, data string }{
		{"Linux", linuxArch, "linux"},
		{"Linux", "test-arch", "extra"},
	} {
		path, err := share.GuestAgent(guest.os, guest.arch)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != guest.data {
			t.Fatalf("%s/%s: read %q, %v", guest.os, guest.arch, data, err)
		}
	}
	if path, err := share.GuestAgent("Linux", "missing"); err == nil {
		t.Fatalf("used %s for an unbundled architecture", path)
	}

	// A different build can reuse the same profile with the old file on disk.
	delete(files, extraFile)
	share, err = writeShare(dir, files)
	if err != nil {
		t.Fatal(err)
	}
	if path, err := share.GuestAgent("Linux", "test-arch"); err == nil {
		t.Fatalf("used %s after the build stopped bundling it", path)
	}
}
