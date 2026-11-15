package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCLISessionStoreRecoveryKeepsTranscriptAndChangesEpoch(t *testing.T) {
	s, id, project := metadataFixture(t)
	old, err := s.cliSessions.record(s.profile, id)
	if err != nil {
		t.Fatal(err)
	}
	if old.CLIID == id {
		t.Fatal("public session ID selected the runtime transcript")
	}
	old.Turns = []cliSessionTurn{{ID: uuid.NewString(), Prompt: "retained prompt", Text: "retained reply", Status: "completed"}}
	old.Resume = true
	old.changed()
	path := cliSessionRecordPath(s.profile, id)
	if err = writePrivate(path, old); err != nil {
		t.Fatal(err)
	}
	s.cliSessions = cliSessionStore{}
	restored, err := s.cliSessions.record(s.profile, id)
	if err != nil || restored.Generation == old.Generation || len(restored.Turns) != 1 || restored.Turns[0].Text != "retained reply" {
		t.Fatal("catalog recovery changed transcript or reused epoch", restored, err)
	}
	if _, err = s.cliSessions.create(s.profile, project, id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.cliSessions.create(s.profile, filepath.Join(project, "another"), id); err == nil {
		t.Fatal("session ID was reused for a different workspace")
	}
	if err = os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.cliSessions.create(s.profile, project, id); err == nil {
		t.Fatal("corrupt history was overwritten")
	}
}

func TestCLILaunchScopeAndAccountBinding(t *testing.T) {
	for _, command := range []string{"claude --dangerously-skip-permissions", "ccs a;id", "ccs --yes", "bash -c anything", "claude | other"} {
		if _, err := cliLaunchArgs(command, claudeSelection{}); err == nil {
			t.Fatal("accepted launcher", command)
		}
	}
	for _, command := range []string{"claude", "ccs klink"} {
		if _, err := cliLaunchArgs(command, claudeSelection{}); err != nil {
			t.Fatal(err)
		}
	}
	id := strings.Repeat("a", 24)
	selection := claudeSelection{ID: id, GuestID: id, Subject: strings.Repeat("b", 64)}
	if _, err := cliLaunchArgs("ccs klink", selection); err == nil {
		t.Fatal("CCS accepted an Eden-managed account")
	}
	args, err := cliLaunchArgs("claude", selection)
	if err != nil || len(args) != 5 || args[4] != id {
		t.Fatal(args, err)
	}
	selection.Subject = "invalid"
	if _, err := cliLaunchArgs("claude", selection); err == nil {
		t.Fatal("invalid account binding accepted")
	}
}

func TestCLIMetadataCannotCrossAccountUnlockEpoch(t *testing.T) {
	s, id, _ := metadataFixture(t)
	epoch, _ := s.account.access()
	old := context.WithValue(context.Background(), cliEpochKey{}, epoch)
	s.account.lock()
	s.account.setKey(make([]byte, 32))
	title := "new title"
	if _, err := s.updateSessionMetadata(id, sessionMetadataInput{Title: &title}, old); err != errCLILocked {
		t.Fatal("previous unlock authorized metadata", err)
	}
	epoch, _ = s.account.access()
	current, cancel := context.WithCancel(context.WithValue(context.Background(), cliEpochKey{}, epoch))
	if _, err := s.updateSessionMetadata(id, sessionMetadataInput{Title: &title}, current); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := s.updateSessionMetadata(id, sessionMetadataInput{Title: &title}, current); err != errCLILocked {
		t.Fatal("cancelled request authorized metadata", err)
	}
}

func TestCLISessionRouteBoundaries(t *testing.T) {
	s, id, _ := metadataFixture(t)
	root := "/api/cli/sessions/" + id
	if w := request(s, "GET", "/api/cli/sessions", "", "", false); w.Code != http.StatusUnauthorized {
		t.Fatal(w.Code)
	}
	if w := request(s, "POST", root+"/metadata", "https://outside.example", `{"pinned":true}`, true); w.Code != http.StatusForbidden {
		t.Fatal(w.Code)
	}
	for _, action := range []string{"messages", "media", "approval", "stop"} {
		if w := request(s, "POST", root+"/"+action, s.origin, `{}`, true); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatal("unexpected desktop command route", action, w.Code)
		}
	}
	s.account.lock()
	if w := request(s, "GET", "/api/cli/sessions", "", "", true); w.Code != http.StatusLocked {
		t.Fatal(w.Code)
	}
}

func TestCLISessionRoutesKeepDesktopContract(t *testing.T) {
	s := testServer(t)
	mux := newDiscoverableMux()
	s.cliSessionRoutes(mux)
	want := []string{
		"GET /api/cli/sessions",
		"GET /api/cli/sessions/{id}",
		"POST /api/cli/sessions/{id}/open",
		"POST /api/cli/sessions/{id}/metadata",
		"POST /api/cli/sessions/{id}/account",
		"POST /api/cli/sessions/{id}/archive",
		"POST /api/cli/sessions/{id}/restore",
		"POST /api/cli/sessions/{id}/purge",
	}
	if !slices.Equal(mux.patterns, want) { t.Fatal("desktop session contract changed", mux.patterns) }
}

func TestCLISessionCatalogDoesNotLoadPhoneRecords(t *testing.T) {
	profile, id := t.TempDir(), uuid.NewString()
	phone := filepath.Join(profile, "mobile-cli-sessions", id+".json")
	if err := writePrivate(phone, map[string]any{"id": id, "workspace": "private", "turns": []any{}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(phone)
	if err != nil {
		t.Fatal(err)
	}
	store := cliSessionStore{}
	list, err := store.summaries(profile, map[string]bool{"private": true}, false)
	if err != nil || len(list) != 0 {
		t.Fatal("loaded an out-of-scope phone record", list, err)
	}
	after, err := os.ReadFile(phone)
	if err != nil || string(before) != string(after) {
		t.Fatal("phone data changed", err)
	}
	if _, err := store.create(profile, "project", "../escape"); err == nil {
		t.Fatal("invalid public ID accepted")
	}
	if _, err := os.Stat(filepath.Join(profile, "cli-sessions")); !os.IsNotExist(err) {
		t.Fatal("invalid creation wrote to disk", err)
	}
}

func TestCLISessionSnapshotOmitsPrivateAccountBinding(t *testing.T) {
	record := cliSessionRecord{Account: claudeSelection{ID: "account", GuestID: "private-guest", Subject: "private-subject"}, GuestConfig: "private-config"}
	raw, err := json.Marshal(record.snapshot())
	if err != nil || strings.Contains(string(raw), "private-") {
		t.Fatal(string(raw), err)
	}
}
