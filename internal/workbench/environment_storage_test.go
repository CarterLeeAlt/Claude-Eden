package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/sylvinus/agent-vm/internal/eden"
)

func TestStorageFirstSelectionAndMigrationThroughWorkbench(t *testing.T) {
	s := testServer(t)
	const endpoint = "/api/environment/storage"
	query := func() eden.StorageLocation {
		t.Helper()
		r := request(s, "GET", endpoint, s.origin, "", true)
		var loc eden.StorageLocation
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &loc) != nil {
			t.Fatalf("%d %s", r.Code, r.Body.String())
		}
		return loc
	}
	previous := query()
	if previous.HasEnvironment {
		t.Fatal("fresh profile reported an environment")
	}
	target := filepath.Join(t.TempDir(), "Eden 空间")
	write := func(target string, prior eden.StorageLocation) Job {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"directory": target, "previous": prior.Directory, "migrate": prior.HasEnvironment})
		r := request(s, "POST", endpoint, s.origin, string(body), true)
		if r.Code != http.StatusOK {
			t.Fatal(r.Code, r.Body.String())
		}
		return awaitJobIdle(t, &s.jobs)
	}
	if job := write(target, previous); job.Error != "" {
		t.Fatal(job)
	}
	location := query()
	if system, err := s.installedLinuxSystem(); err != nil || system != "" {
		t.Fatalf("storage selection bound an uncreated VM: %q %v", system, err)
	}
	real, err := filepath.EvalSymlinks(target)
	if err != nil || location.Directory != real || !location.Custom {
		t.Fatal(location, real, err)
	}
	for _, path := range []string{target, filepath.Dir(target), filepath.Join(target, "state")} {
		if _, err := validateWorkspace(path, s.profile); err == nil {
			t.Fatal("environment storage exposed as project", path)
		}
	}
	project := t.TempDir()
	if err := s.rememberSharedProject(project); err != nil {
		t.Fatal(err)
	}
	if job := write(filepath.Join(project, "private-disks"), location); job.Error == "" {
		t.Fatal("saved project accepted as environment storage")
	}
	staleTarget := filepath.Join(t.TempDir(), "old-window")
	if job := write(staleTarget, previous); job.Error == "" {
		t.Fatal("stale window redirected the active environment", job)
	}
	if _, err := os.Stat(staleTarget); !os.IsNotExist(err) {
		t.Fatal("stale request created a destination", err)
	}
	// A disk added between showing the form and submitting it must now require
	// the migration confirmation. Exercise the subsequent cold move through
	// the same authenticated HTTP operation used by the settings page.
	relativeDisk := filepath.Join("state", "lima", "fixture", "diffdisk")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(target, relativeDisk)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, relativeDisk), []byte("isolated VM disk"), 0600); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "Eden 环境")
	if job := write(second, location); job.Error == "" {
		t.Fatal("unconfirmed migration was accepted")
	}
	from := query()
	if !from.HasEnvironment {
		t.Fatal("disk was not discovered")
	}
	if job := write(second, from); job.Error != "" {
		t.Fatal(job)
	}
	moved := query()
	if !moved.HasEnvironment || moved.Backup != from.Directory {
		t.Fatal(moved)
	}
	for _, root := range []string{moved.Backup, moved.Directory} {
		data, err := os.ReadFile(filepath.Join(root, relativeDisk))
		if err != nil || string(data) != "isolated VM disk" {
			t.Fatal(root, err)
		}
	}
}

func TestStorageChangeRequiresUnlockedSession(t *testing.T) {
	s := testServer(t)
	const endpoint = "/api/environment/storage"
	s.account.lock()
	if r := request(s, "POST", endpoint, s.origin, "{}", true); r.Code != http.StatusLocked {
		t.Fatal("locked user could change storage", r.Code)
	}
}

func TestStorageRecoveryAfterReconnectingLinuxDisk(t *testing.T) {
	s := testServer(t)
	previous, err := eden.GetStorageLocation(s.profile)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "linux-storage")
	if _, err := eden.MoveStorage(context.Background(), s.profile, target, previous, nil); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join("state", "lima", "fixture", "diffdisk")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(target, disk)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, disk), []byte("preserved Linux disk"), 0600); err != nil {
		t.Fatal(err)
	}
	assertSystem := func(system string, wantError bool) {
		t.Helper()
		for _, endpoint := range []string{"/api/account", "/api/status"} {
			response := request(s, http.MethodGet, endpoint, s.origin, "", true)
			var view struct {
				System string `json:"guest_system"`
				Error  string `json:"guest_system_error"`
			}
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &view) != nil || view.System != system {
				t.Fatalf("%s: status=%d system=%q want=%q", endpoint, response.Code, view.System, system)
			}
			if endpoint == "/api/status" && (view.Error != "") != wantError {
				t.Fatalf("%s: unexpected storage error %q", endpoint, view.Error)
			}
		}
	}
	assertSystem("debian-13", false)
	offline := target + "-offline"
	if err := os.Rename(target, offline); err != nil {
		t.Fatal(err)
	}
	response := request(s, http.MethodGet, "/api/environment/storage", s.origin, "", true)
	var view struct {
		Recovery storageRecoveryView `json:"recovery"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &view) != nil || !view.Recovery.Required || view.Recovery.Error == "" {
		t.Fatalf("missing disk did not enter recovery: %d %s", response.Code, response.Body.String())
	}
	assertSystem("", true)
	for _, endpoint := range []string{"/api/start", "/api/environment/storage"} {
		response = request(s, http.MethodPost, endpoint, s.origin, "{}", true)
		if response.Code != http.StatusConflict {
			t.Fatalf("recovery admitted %s: %d %s", endpoint, response.Code, response.Body.String())
		}
	}
	if response := request(s, http.MethodGet, "/api/account", s.origin, "", true); response.Code != http.StatusOK {
		t.Fatalf("recovery blocked local account access: %d %s", response.Code, response.Body.String())
	}
	recover := func() Job {
		t.Helper()
		response := request(s, http.MethodPost, "/api/environment/storage/recover", s.origin, "{}", true)
		if response.Code != http.StatusOK {
			t.Fatalf("recovery request: %d %s", response.Code, response.Body.String())
		}
		return awaitJobIdle(t, &s.jobs)
	}
	if job := recover(); job.Error == "" || !s.storageRecoveryStatus().Required {
		t.Fatal("missing disk was reported recovered", job)
	}
	assertSystem("", true)
	if err := os.Rename(offline, target); err != nil {
		t.Fatal(err)
	}
	if job := recover(); job.Error != "" || s.storageRecoveryStatus().Required {
		t.Fatal("reconnected disk did not recover", job, s.storageRecoveryStatus())
	}
	assertSystem("debian-13", false)
	location, err := eden.GetStorageLocation(s.profile)
	if err != nil || !location.HasEnvironment || !location.Custom {
		t.Fatal(location, err)
	}
	if data, err := os.ReadFile(filepath.Join(s.profile, disk)); err != nil || string(data) != "preserved Linux disk" {
		t.Fatalf("recovery changed the original disk: %q %v", data, err)
	}
	response = request(s, http.MethodPost, "/api/environment/storage/recover", s.origin, "{}", true)
	if response.Code != http.StatusConflict {
		t.Fatalf("recovery restarted after success: %d %s", response.Code, response.Body.String())
	}
}
