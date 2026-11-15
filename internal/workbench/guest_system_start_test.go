package workbench

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestUnsupportedGuestSystemCannotStartDebian(t *testing.T) {
	s := testServer(t)
	for _, system := range []string{"windows-11", "debian-12", "ubuntu-22.04", "ubuntu-24.04", "unknown"} {
		response := request(s, http.MethodPost, "/api/start", s.origin, `{"guest_system":"`+system+`"}`, true)
		if response.Code != http.StatusBadRequest || s.jobs.snapshot().ID != "" {
			t.Fatalf("%s: got %d, job %v", system, response.Code, s.jobs.snapshot())
		}
	}
}

func TestInvalidInstallationPreventsImplicitDebianStart(t *testing.T) {
	for _, record := range []string{`{"version":1,"system":"ubuntu-22.04","id":"` + uuid.NewString() + `"}`, `{"version":1,"system":"debian-13","id":"invalid"}`, `{"broken"`} {
		s := testServer(t)
		path := filepath.Join(s.profile, "guest-install.json")
		if err := os.WriteFile(path, []byte(record), 0600); err != nil {
			t.Fatal(err)
		}
		for _, body := range []string{"{}", `{"guest_system":"debian-13"}`} {
			w := request(s, "POST", "/api/start", s.origin, body, true)
			if w.Code != http.StatusConflict || s.jobs.snapshot().ID != "" {
				t.Fatalf("created a job from invalid installation (%s): %d", body, w.Code)
			}
		}
		if err := s.startEnvironment(false); err == nil {
			t.Fatal("indirect start accepted invalid installation")
		}
		retained, err := os.ReadFile(path)
		if err != nil || string(retained) != record {
			t.Fatal("invalid installation record was replaced", err)
		}
	}
}
