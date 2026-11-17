package guestos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/hostimage"
	"github.com/sylvinus/agent-vm/internal/vmname"
)

func TestInstallationKeepsChoiceAndDiskBinding(t *testing.T) {
	p := t.TempDir()
	first := Installation{Version: 1, System: "debian-13", ID: uuid.New(), Source: hostimage.Plan{
		Host:    hostimage.Identity{System: "debian-13", Version: "13", Arch: "amd64"},
		Version: "13", Format: "qcow2", URL: "https://cloud.debian.org/system.img", Digest: "sha256:" + strings.Repeat("a", 64),
	}}
	if err := SaveInstallation(p, first); err != nil {
		t.Fatal(err)
	}
	if err := SaveInstallation(p, first); err != nil {
		t.Fatal(err)
	}
	other := first
	other.ID = uuid.New()
	if err := SaveInstallation(p, other); err == nil {
		t.Fatal("overwrote an active installation")
	}
	if unstarted, err := UnstartedInstallation(p, first); err != nil || !unstarted {
		t.Fatal("fresh binding was treated as a created disk", err)
	}
	disk := filepath.Join(p, "state", "lima", vmname.Building)
	if err := os.MkdirAll(disk, 0700); err != nil {
		t.Fatal(err)
	}
	if unstarted, err := UnstartedInstallation(p, first); err != nil || unstarted {
		t.Fatal("forgot a partially installed VM", err)
	}
	got, err := LoadInstallation(p)
	if err != nil || got == nil || *got != first {
		t.Fatalf("changed original selection: %v %v", got, err)
	}
	if err := SaveInstallation(p, other); err == nil {
		t.Fatal("replaced an installation after disk creation")
	}
}
