package vm

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/guestprofile"
	"github.com/sylvinus/agent-vm/internal/hostimage"
)

func TestHostLinuxImageHasNoFallbackOrHostMounts(t *testing.T) {
	l := newTestLima(t)
	directory := t.TempDir()
	config := filepath.Join(directory, "config.json")
	data, _ := json.Marshal(eden.DefaultConfig())
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(eden.ConfigEnv, config)
	plan := hostimage.Plan{Host: hostimage.Identity{System: "debian-13", Version: "13", Arch: "amd64"}, Version: "13", Format: "qcow2", Path: filepath.Join(directory, "debian.img"), Digest: "sha256:" + strings.Repeat("a", 64)}
	if err := hostimage.WriteLinuxTemplate(directory, plan); err != nil {
		t.Fatal(err)
	}
	template, err := hostimage.LinuxTemplate(directory)
	if err != nil {
		t.Fatal(err)
	}
	none := []Mount{}
	if err := l.Create(t.Context(), "x", template, Settings{Mounts: &none, NoContainerd: true}, io.Discard); err != nil {
		t.Fatal(err)
	}
	instance, err := l.inspect(t.Context(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(instance.Config.Images) != 1 || instance.Config.Images[0].Location != plan.Path || string(instance.Config.Images[0].Digest) != plan.Digest {
		t.Fatalf("unexpected image fallback: %+v", instance.Config.Images)
	}
	if len(instance.Config.Mounts) != 0 {
		t.Fatal("host directories were mounted")
	}
	u := instance.Config.User
	if *u.Name != "developer" || *u.Comment != "Development user" || *u.Home != "/home/developer" || *u.UID != 1000 || *u.Shell != "/bin/bash" || !*u.PasswordlessSudo {
		t.Fatalf("incomplete development identity: %+v", u)
	}
	changed := plan
	changed.Path = filepath.Join(directory, "different.img")
	if err := hostimage.WriteLinuxTemplate(directory, changed); err == nil {
		t.Fatal("replaced the durable image binding")
	}
}

func TestLimaSavedGuestProfileSurvivesBaseCloneAndEdit(t *testing.T) {
	l := newTestLima(t)
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	data, err := json.Marshal(eden.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := guestprofile.Create(dir, guestprofile.Options{Mode: "manual", Hostname: "X", Username: "alex", BrowserUsername: "web", WorkHome: "/home/projects"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(eden.ConfigEnv, config)
	none := []Mount{}
	if err := l.Create(t.Context(), "base", "template:debian-13", Settings{Mounts: &none}, io.Discard); err != nil {
		t.Fatal(err)
	}
	source, err := l.inspect(t.Context(), "base")
	if err != nil {
		t.Fatal(err)
	}
	if *source.Config.User.Name != p.Username || *source.Config.User.Home != p.WorkHomePath() || source.Config.GuestIdentity.Hostname != "X" {
		t.Fatal("creation ignored the saved identity")
	}
	if err := l.Clone(t.Context(), "base", "project"); err != nil {
		t.Fatal(err)
	}
	clone, err := l.inspect(t.Context(), "project")
	if err != nil {
		t.Fatal(err)
	}
	identity := *clone.Config.GuestIdentity
	if identity.Hostname != "X" || identity.UUID == source.Config.GuestIdentity.UUID || identity.MACAddress == source.Config.GuestIdentity.MACAddress || *clone.Config.User.Name != p.Username {
		t.Fatal("clone did not keep accounts and assign an independent machine identity")
	}
	if err := l.Edit(t.Context(), "project", Settings{CPUs: 2}); err != nil {
		t.Fatal(err)
	}
	again, err := l.inspect(t.Context(), "project")
	if err != nil || *again.Config.GuestIdentity != identity {
		t.Fatal("resource edit changed identity", err)
	}
}
