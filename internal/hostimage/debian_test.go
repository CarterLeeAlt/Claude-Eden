package hostimage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/hostimage"
)

func TestDebian13PinnedImageAndPersistentTemplate(t *testing.T) {
	template, err := hostimage.ReadImageTemplate("debian-13")
	if err != nil {
		t.Fatal(err)
	}
	identity := hostimage.Identity{System: "debian-13", Version: "13", Arch: "amd64"}
	state := t.TempDir()
	plan, err := hostimage.Resolve(identity, state, template)
	if err != nil || !strings.Contains(plan.URL, "/trixie/20260712-2537/") || len(plan.Digest) != 135 {
		t.Fatalf("source: %+v %v", plan, err)
	}
	plan.Path = filepath.Join(state, "debian.img")
	if err := hostimage.WriteLinuxTemplate(state, plan); err != nil {
		t.Fatal(err)
	}
	saved, err := hostimage.Load(filepath.Join(state, "guest-media"))
	if err != nil || saved != plan {
		t.Fatalf("persistent media: %+v %v", saved, err)
	}
	path, err := hostimage.LinuxTemplate(state)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	var config struct{ Images []map[string]string }
	if err != nil || json.Unmarshal(data, &config) != nil || len(config.Images) != 1 || config.Images[0]["location"] != plan.Path {
		t.Fatalf("template changed source or added fallbacks: %s %v", data, err)
	}
	plan.URL = "https://cloud.debian.org.evil.test/debian.img"
	if err := plan.Validate(); err == nil {
		t.Fatal("accepted an untrusted Debian image source")
	}
}

func TestDebianChoiceDoesNotIgnoreMetadataLoss(t *testing.T) {
	for _, removed := range []string{"host-system.yaml", "host-system-source.json"} {
		state := t.TempDir()
		template, _ := hostimage.ReadImageTemplate("debian-13")
		plan, err := hostimage.Resolve(hostimage.Identity{System: "debian-13", Version: "13", Arch: "amd64"}, state, template)
		if err != nil {
			t.Fatal(err)
		}
		plan.Path = filepath.Join(state, "debian.img")
		if err := hostimage.WriteLinuxTemplate(state, plan); err != nil {
			t.Fatal(err)
		}
		if system, err := hostimage.LinuxSystem(state); err != nil || system != "debian-13" {
			t.Fatalf("identity: %q %v", system, err)
		}
		if err := os.Remove(filepath.Join(state, "guest-media", removed)); err != nil {
			t.Fatal(err)
		}
		if _, err := hostimage.LinuxTemplate(state); err == nil {
			t.Fatalf("silently fell back after losing %s", removed)
		}
	}
}
