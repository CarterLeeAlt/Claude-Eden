package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/guestos"
	"github.com/sylvinus/agent-vm/internal/hostimage"
	"github.com/sylvinus/agent-vm/internal/vmname"
)

func TestDebianGuestChoicesMatchSupportedHostArchitectures(t *testing.T) {
	for _, host := range []struct {
		platform, arch string
		available      bool
	}{
		{"darwin", "arm64", false}, {"windows", "amd64", true}, {"linux", "amd64", false},
		{"darwin", "amd64", false}, {"windows", "arm64", false}, {"linux", "arm64", false},
	} {
		if got := linuxGuestSystems(host.platform, host.arch); (len(got) == 1 && got[0] == "debian-13") != host.available {
			t.Fatalf("%s/%s: %v", host.platform, host.arch, got)
		}
	}
}

func TestDebianCatalogPlanPreservesArchitectureAndResumeChoice(t *testing.T) {
	plan, err := linuxGuestImagePlan(t.Context(), t.TempDir(), "debian-13", "amd64")
	if err != nil || plan.Host.Arch != "amd64" || !strings.Contains(plan.URL, "debian-13-genericcloud-amd64-20260712-2537.qcow2") || !strings.HasPrefix(plan.Digest, "sha512:") {
		t.Fatalf("amd64: %+v %v", plan, err)
	}
	for _, identity := range [][2]string{{"debian-13", "arm64"}, {"debian-12", "amd64"}, {"ubuntu-22.04", "amd64"}, {"ubuntu-24.04", "amd64"}} {
		if _, err := linuxGuestImagePlan(t.Context(), t.TempDir(), identity[0], identity[1]); err == nil {
			t.Fatalf("accepted unsupported image %v", identity)
		}
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return
	}
	s := testServer(t)
	plan, err = linuxGuestImagePlan(t.Context(), s.profile, "debian-13", runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	install := guestos.Installation{Version: 1, ID: uuid.New(), System: "debian-13", Source: plan}
	if err := guestos.SaveInstallation(s.profile, install); err != nil {
		t.Fatal(err)
	}
	view, err := s.guestInstallView()
	if err != nil || view.Pending == nil || view.Pending.Source != plan || !view.Fresh {
		t.Fatalf("cross-host resume: %+v %v", view, err)
	}
	if err := os.MkdirAll(filepath.Join(s.profile, "state", "lima", vmname.Building), 0700); err != nil {
		t.Fatal(err)
	}
	view, err = s.guestInstallView()
	if err != nil || view.Pending == nil || view.Pending.ID != install.ID || view.Fresh {
		t.Fatalf("started disk lost its binding: %+v %v", view, err)
	}
}

func TestDebianPendingInstallerRequiresTrustedDesktop(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("Windows amd64 product admission")
	}
	for _, trusted := range []bool{false, true} {
		s := testServer(t)
		// Keep accepted work from touching a real VM or network in this admission test.
		s.op.Lock()
		r := httptest.NewRequest(http.MethodPost, "/api/start", strings.NewReader(`{"guest_system":"debian-13"}`))
		r.Header.Set("Content-Type", "application/json")
		if trusted {
			r.AddCookie(&http.Cookie{Name: s.cookieName() + "-desktop", Value: trustedViewProof(s.token)})
		}
		w := httptest.NewRecorder()
		s.beginGuestInstall(w, r, guestInstallRequest{System: "debian-13"})
		job := s.jobs.snapshot()
		s.jobs.cancelRunning()
		s.op.Unlock()
		s.jobs.workers.Wait()
		if trusted {
			var response Job
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Kind != "guest-install" || job.Kind != "guest-install" {
				t.Fatalf("pending installer admission failed: %d %s", w.Code, w.Body.String())
			}
		} else if w.Code == http.StatusOK {
			t.Fatal("untrusted page started installation")
		}
	}
}

func TestDebianImageBindingCannotBeReplaced(t *testing.T) {
	s := testServer(t)
	plan, err := linuxGuestImagePlan(t.Context(), s.profile, "debian-13", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	plan.Path = filepath.Join(s.profile, "debian.img")
	if err := hostimage.WriteLinuxTemplate(filepath.Join(s.profile, "state"), plan); err != nil {
		t.Fatal(err)
	}
	plan.Path = filepath.Join(s.profile, "different.img")
	if err := hostimage.WriteLinuxTemplate(filepath.Join(s.profile, "state"), plan); err == nil {
		t.Fatal("replaced an existing Debian image binding")
	}
}

func TestInstalledGuestSystemMatchesExistingEnvironment(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("Windows amd64 product environment")
	}
	for _, test := range []struct {
		name, instance, system     string
		material, pending, corrupt bool
	}{
		{name: "fresh"},
		{name: "downloaded Debian", material: true},
		{name: "pending Debian", material: true, pending: true},
		{name: "partial Debian", instance: vmname.Building, system: "debian-13"},
		{name: "installed Debian", instance: "base", system: "debian-13"},
		{name: "partial bound Debian", instance: vmname.Building, material: true, pending: true, system: "debian-13"},
		{name: "installed bound Debian", instance: "base", material: true, system: "debian-13"},
		{name: "damaged Debian", instance: "base", material: true, corrupt: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testServer(t)
			state := filepath.Join(s.profile, "state")
			if test.material {
				plan, err := linuxGuestImagePlan(t.Context(), s.profile, "debian-13", runtime.GOARCH)
				if err != nil {
					t.Fatal(err)
				}
				plan.Path = filepath.Join(state, "debian.img")
				if err := hostimage.WriteLinuxTemplate(state, plan); err != nil {
					t.Fatal(err)
				}
				if test.pending {
					if err := guestos.SaveInstallation(s.profile, guestos.Installation{Version: 1, ID: uuid.New(), System: "debian-13", Source: plan}); err != nil {
						t.Fatal(err)
					}
				}
				if test.corrupt {
					if err := os.WriteFile(filepath.Join(state, "guest-media", "host-system.yaml"), []byte("invalid template"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if test.instance != "" {
				if err := os.MkdirAll(filepath.Join(state, "lima", test.instance), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, endpoint := range []string{"/api/account", "/api/status"} {
				response := request(s, http.MethodGet, endpoint, s.origin, "", true)
				var view struct {
					System string `json:"guest_system"`
					Error  string `json:"guest_system_error"`
				}
				if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &view) != nil || view.System != test.system {
					t.Fatalf("%s: status=%d system=%q want=%q", endpoint, response.Code, view.System, test.system)
				}
				if endpoint == "/api/status" && (view.Error != "") != test.corrupt {
					t.Fatalf("%s: unexpected image state error %q", endpoint, view.Error)
				}
			}
			view, err := s.guestInstallView()
			if err != nil || view.Fresh != (test.instance == "") || (view.Pending != nil) != test.pending {
				t.Fatalf("selection state: %+v %v", view, err)
			}
			body, err := json.Marshal(view)
			if err != nil || strings.Contains(string(body), "reselect") {
				t.Fatalf("unexpected reselection contract: %s %v", body, err)
			}
		})
	}
}

func TestDefaultAndPendingDebianStartRoutes(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("Windows amd64 product start routes")
	}
	for _, pending := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			s := testServer(t)
			system, kind := "debian-13", "start"
			if pending {
				kind = "guest-install"
				plan, err := linuxGuestImagePlan(t.Context(), s.profile, system, runtime.GOARCH)
				if err != nil {
					t.Fatal(err)
				}
				if err := guestos.SaveInstallation(s.profile, guestos.Installation{Version: 1, ID: uuid.New(), System: system, Source: plan}); err != nil {
					t.Fatal(err)
				}
			}
			body := "{}"
			if explicit {
				body = `{"guest_system":"` + system + `"}`
			}
			// Run real admission, then cancel queued work before VM/network execution.
			s.op.Lock()
			r := httptest.NewRequest(http.MethodPost, "/api/start", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.AddCookie(&http.Cookie{Name: s.cookieName() + "-desktop", Value: trustedViewProof(s.token)})
			response := httptest.NewRecorder()
			s.start(response, r)
			job := s.jobs.snapshot()
			s.jobs.cancelRunning()
			s.op.Unlock()
			s.jobs.workers.Wait()
			if response.Code != http.StatusOK || job.Kind != kind {
				t.Fatalf("pending=%v explicit=%v: %d kind=%q", pending, explicit, response.Code, job.Kind)
			}
		}
	}
}
