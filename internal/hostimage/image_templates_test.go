package hostimage

import (
	"path/filepath"
	"testing"
)

func TestImageTemplatesResolveProductArchitecture(t *testing.T) {
	data, err := ReadImageTemplate("debian-13")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Resolve(Identity{System: "debian-13", Version: "13", Arch: "amd64"}, filepath.Join(t.TempDir(), "cache"), data)
	if err != nil || plan.Digest == "" || plan.Host.System != "debian-13" || plan.Host.Arch != "amd64" {
		t.Fatalf("pinned image: %+v, %v", plan, err)
	}
	for _, system := range []string{"", "../debian-13", "debian-12", "windows-11", "ubuntu-22.04", "ubuntu-24.04"} {
		if _, err := ReadImageTemplate(system); err == nil {
			t.Fatalf("accepted unsupported template %q", system)
		}
	}
}
