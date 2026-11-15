package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalConnectionKeepsSessionAndOriginBoundary(t *testing.T) {
	s := testServer(t)
	s.gateway = newClientGateway()
	defer s.gateway.close()
	s.account.lock()
	if got := request(s, "GET", "/api/client/hosts", "", "", false).Code; got != 401 {
		t.Fatalf("unauthenticated host list: %d", got)
	}
	if got := request(s, "GET", "/api/client/context", "", "", true).Code; got != 200 {
		t.Fatalf("locked local account prevents connection management: %d", got)
	}
	if got := request(s, "POST", "/api/client/disconnect", "https://untrusted.example", "{}", true).Code; got != 403 {
		t.Fatalf("cross-origin connection mutation: %d", got)
	}
	if got := request(s, "GET", "/api/client/hosts", "", "", true).Code; got != 404 {
		t.Fatalf("removed host management is available: %d", got)
	}
	r := httptest.NewRequest("POST", s.origin+"/api/stop", strings.NewReader("{}"))
	r.Header.Set("Origin", s.origin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(connectionHeader, "previous-controller-connection")
	r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.session})
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("stale request reached local runtime: %d", w.Code)
	}
}

func TestControllerIdentityPersistsAndCorruptionDoesNotReplaceIt(t *testing.T) {
	profile := t.TempDir()
	first, err := ensureControllerInstance(profile)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ensureControllerInstance(profile)
	if err != nil || first != again {
		t.Fatal("identity changed on reconnect", err)
	}
	other, err := ensureControllerInstance(t.TempDir())
	if err != nil || first == other {
		t.Fatal("different instances share an identity", err)
	}
	path := filepath.Join(profile, "controller-instance.json")
	corrupt := []byte(`{"id":"invalid"}`)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureControllerInstance(profile); err == nil {
		t.Fatal("corrupt identity was accepted")
	}
	retained, _ := os.ReadFile(path)
	if string(retained) != string(corrupt) {
		t.Fatal("corrupt identity was silently replaced")
	}
}

func TestProjectDirectoryPickerHandlesEmptyProjectsAndBlocksPrivateSymlinks(t *testing.T) {
	s := testServer(t)
	empty := t.TempDir()
	w := request(s, "GET", "/api/projects/directories?path="+url.QueryEscape(empty), "", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Directories []any `json:"directories"`
		Selectable  bool  `json:"selectable"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Directories) != 0 || !result.Selectable {
		t.Fatal("empty project unavailable", w.Body.String())
	}
	link := filepath.Join(t.TempDir(), "private-link")
	paths := []string{s.profile}
	if err := os.Symlink(s.profile, link); err == nil {
		paths = append(paths, link)
	}
	for _, path := range paths {
		w := request(s, "GET", "/api/projects/directories?path="+url.QueryEscape(path), "", "", true)
		if w.Code != 403 {
			t.Fatalf("private directory was listed: %d", w.Code)
		}
	}
}
