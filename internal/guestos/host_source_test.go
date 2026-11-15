package guestos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/hostimage"
)

func TestHostSourcePublishesOneVerifiedMediaReceipt(t *testing.T) {
	profile := t.TempDir()
	source := hostimage.Plan{Host: hostimage.Identity{System: "debian-13", Version: "13", Arch: "amd64"}, Version: "13", Format: "qcow2", URL: "https://cloud.debian.org/example.img", Digest: "sha256:" + strings.Repeat("a", 64), Bytes: 10}
	install := Installation{Version: 1, ID: uuid.New(), System: "debian-13", Source: source}
	if err := SaveInstallation(profile, install); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(profile, installRecord))
	ready := source
	ready.Path = filepath.Join(profile, "system.img")
	ready.Modified = 123
	if err := RecordPreparedMedia(profile, install.ID, ready); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadInstallation(profile)
	if err != nil || loaded.Media != ready.Path || loaded.Source != ready {
		t.Fatalf("loaded: %+v %v", loaded, err)
	}
	if err := SaveInstallation(profile, *loaded); err != nil {
		t.Fatal(err)
	}
	if err := RecordPreparedMedia(profile, install.ID, ready); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(profile, installRecord))
	if string(original) != string(after) {
		t.Fatal("changed original installation choice")
	}
	changed := ready
	changed.Host.Arch = "arm64"
	if err := RecordPreparedMedia(profile, install.ID, changed); err == nil {
		t.Fatal("accepted another architecture")
	}
	if err := RecordPreparedMedia(profile, uuid.New(), ready); err == nil {
		t.Fatal("accepted another installation")
	}
	changed = ready
	changed.Path = filepath.Join(profile, "other.img")
	if err := RecordPreparedMedia(profile, install.ID, changed); err == nil {
		t.Fatal("replaced published media")
	}
}

func TestLinuxInstallationKeepsItsChoiceAfterBaseInstallation(t *testing.T) {
	for _, name := range []string{"base-building", "base"} {
		t.Run(name, func(t *testing.T) {
			profile := t.TempDir()
			install := Installation{
				Version: 1, ID: uuid.New(), System: "debian-13",
				Source: hostimage.Plan{
					Host:    hostimage.Identity{System: "debian-13", Version: "13", Arch: "amd64"},
					Version: "13", Format: "qcow2", Path: filepath.Join(profile, "debian.img"),
					Digest: "sha256:" + strings.Repeat("a", 64),
				},
			}
			if err := SaveInstallation(profile, install); err != nil {
				t.Fatal(err)
			}
			disk := filepath.Join(profile, "state", "lima", name)
			if err := os.MkdirAll(disk, 0700); err != nil {
				t.Fatal(err)
			}
			unstarted, err := UnstartedInstallation(profile, install)
			if err != nil || unstarted {
				t.Fatalf("existing base was treated as an unstarted choice: %v %v", unstarted, err)
			}
			if err := FinishLinuxInstallation(profile, install.ID); err == nil {
				t.Fatal("finished without a durable Linux choice")
			}
			if pending, err := LoadInstallation(profile); err != nil || pending == nil {
				t.Fatal("lost installation recovery information", err)
			}
			if err := hostimage.WriteLinuxTemplate(filepath.Join(profile, "state"), install.Source); err != nil {
				t.Fatal(err)
			}
			if err := FinishLinuxInstallation(profile, install.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(disk); err != nil {
				t.Fatal("finishing the installation affected its disk", err)
			}
		})
	}
}
