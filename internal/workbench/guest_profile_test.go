package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/guestprofile"
)

func guestProfileRequest(s *Server, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, s.origin+path, strings.NewReader(body))
	r.Header.Set("Origin", s.origin)
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.session})
	r.AddCookie(&http.Cookie{Name: s.cookieName() + "-desktop", Value: trustedViewProof(s.token)})
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, r)
	return w
}

func TestGuestProfilePreviewDoesNotPersistAndInstallKeepsSelection(t *testing.T) {
	s := testServer(t)
	r := guestProfileRequest(s, "GET", "/api/environment/guest-profile", "")
	var state struct {
		Profile  *guestprofile.Profile `json:"profile"`
		Editable bool                  `json:"editable"`
		Defaults guestprofile.Options  `json:"defaults"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &state) != nil || !state.Editable || state.Profile != nil || state.Defaults.Username != "developer" {
		t.Fatal(r.Code, r.Body.String())
	}
	r = guestProfileRequest(s, "POST", "/api/environment/guest-profile/preview", `{"mode":"random"}`)
	var options guestprofile.Options
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &options) != nil || options.Mode != "random" {
		t.Fatal(r.Code, r.Body.String())
	}
	if p, err := guestprofile.Load(s.profile); err != nil || p != nil {
		t.Fatal("preview changed saved profile", err)
	}
	if err := s.prepareGuestProfile(&options); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(s.profile, guestprofile.Filename))
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []*guestprofile.Options{nil, &options} {
		if err := s.prepareGuestProfile(selection); err != nil {
			t.Fatal("resume rejected saved selection", err)
		}
	}
	changed := options
	changed.Username = "another"
	if err := s.prepareGuestProfile(&changed); err == nil {
		t.Fatal("changed creation identity during resume")
	}
	after, err := os.ReadFile(filepath.Join(s.profile, guestprofile.Filename))
	if err != nil || string(before) != string(after) {
		t.Fatal("resume rewrote saved profile", err)
	}
	r = guestProfileRequest(s, "GET", "/api/environment/guest-profile", "")
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &state) != nil || state.Editable || state.Profile == nil || state.Profile.Options != options {
		t.Fatal("saved creation options were not shown", r.Body.String())
	}
}

func TestGuestProfilePreservesExistingEnvironment(t *testing.T) {
	s := testServer(t)
	disk := filepath.Join(s.profile, "state", "lima", "base", "diffdisk")
	if err := os.MkdirAll(filepath.Dir(disk), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disk, []byte("existing data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareGuestProfile(nil); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareGuestProfile(&guestprofile.Options{Mode: "random"}); err == nil {
		t.Fatal("existing disk accepted different identity")
	}
	if p, err := guestprofile.Load(s.profile); err != nil || p != nil {
		t.Fatal("existing environment acquired a new profile", err)
	}
	b, err := os.ReadFile(disk)
	if err != nil || string(b) != "existing data" {
		t.Fatal("existing disk changed", err)
	}
}

func TestGuestProfileRoutesRejectInvalidAndUnauthenticatedInput(t *testing.T) {
	s := testServer(t)
	for _, endpoint := range []string{"/api/environment/guest-profile", "/api/environment/guest-profile/preview"} {
		method, body := "GET", ""
		if endpoint != "/api/environment/guest-profile" {
			method, body = "POST", "{}"
		}
		if r := request(s, method, endpoint, s.origin, body, false); r.Code != 401 {
			t.Fatal("untrusted profile access", r.Code)
		}
	}
	for _, body := range []string{`{"username":"root"}`, `{"graphics_backend":"metal"}`, `{"gpu_backend":"virgl"}`, `{"display_width":1}`, `{"mode":"other"}`} {
		if r := guestProfileRequest(s, "POST", "/api/environment/guest-profile/preview", body); r.Code == 200 {
			t.Fatal("invalid profile accepted", body)
		}
	}
}
