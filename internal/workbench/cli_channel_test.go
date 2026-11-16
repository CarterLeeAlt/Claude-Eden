//go:build !windows

package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"al.essio.dev/pkg/shellescape"
	"github.com/google/uuid"
)

func TestCLIChannelProtocol(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	cmd := exec.Command(python, "-I", "cli_channel_test.py")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("channel protocol: %v\n%s", err, output)
	}
}

func TestSessionCatalogActivity(t *testing.T) {
	s, id, project := metadataFixture(t)
	record, err := s.cliSessions.record(s.profile, id)
	if err != nil {
		t.Fatal(err)
	}
	record.Terminal = true
	record.Active = true
	if err := writePrivate(cliSessionRecordPath(s.profile, id), record); err != nil {
		t.Fatal(err)
	}
	s.cliSessions.catalog = nil
	list, err := s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false)
	if err != nil || len(list) != 1 || !list[0].Active {
		t.Fatal("catalog lost task activity", list, err)
	}
	record.Active = false
	if err := writePrivate(cliSessionRecordPath(s.profile, id), record); err != nil {
		t.Fatal(err)
	}
	s.cliSessions.catalog = nil
	list, err = s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false)
	if err != nil || list[0].Active {
		t.Fatal("completed turn stayed active", list, err)
	}
}

func TestImportedCLIUUIDIsIndependentOfPublicSessionID(t *testing.T) {
	profile, id := t.TempDir(), uuid.NewString()
	record := &cliSessionRecord{cliSession: cliSession{cliSessionSummary: cliSessionSummary{ID: id, Workspace: "/project"}, Turns: []cliSessionTurn{}}, CLIID: "c4cbf880-92e8-11f1-8596-0242ac120002", Terminal: true}
	if err := writePrivate(cliSessionRecordPath(profile, id), record); err != nil {
		t.Fatal(err)
	}
	var store cliSessionStore
	if _, err := store.record(profile, id); err != nil {
		t.Fatal(err)
	}
	if cliSessionID(record.CLIID) {
		t.Fatal("CLI identity widened the client creation contract")
	}
}

func TestArchivedCLIHistoryIsNotRediscovered(t *testing.T) {
	s, id, project := metadataFixture(t)
	home, _ := filepath.EvalSymlinks(t.TempDir())
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	// This archived history has no live CLI process. Keep the real discovery
	// helper and SSH path independent of a developer's installed Claude CLI.
	cli := "#!/bin/sh\n[ \"$#\" -eq 2 ] && [ \"$1\" = agents ] && [ \"$2\" = --json ] || exit 64\nprintf '[]\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(cli), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("export PATH="+shellescape.Quote(bin)+":/usr/bin:/bin\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cliTestSSH(t, s, home, []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin", "CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude")})
	record, err := s.cliSessions.record(s.profile, id)
	if err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(home, ".claude", "projects", "fixture", record.CLIID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(history), 0700); err != nil {
		t.Fatal(err)
	}
	row, _ := json.Marshal(map[string]any{"cwd": guestPath(project), "type": "user", "uuid": uuid.NewString(), "message": map[string]string{"content": "guest title"}})
	if err := os.WriteFile(history, append(row, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false); err != nil {
		t.Fatal(err)
	}
	s.ready = true
	if err := s.discoverTerminalConversations(context.Background(), []string{project}); err != nil {
		t.Fatal(err)
	}
	if w := request(s, "POST", "/api/cli/sessions/"+id+"/metadata", s.origin, `{"title":"保留用户命名","pinned":true}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := s.discoverTerminalConversations(context.Background(), []string{project}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false)
	if len(list) != 1 || list[0].Title != "保留用户命名" || !list[0].Pinned {
		t.Fatal("discovery overwrote metadata", list)
	}
	if err := s.setSessionArchived(id, true, nil); err != nil {
		t.Fatal(err)
	}
	s.cliSessions = cliSessionStore{}
	if _, err := s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false); err != nil {
		t.Fatal(err)
	}
	if err := s.discoverTerminalConversations(context.Background(), []string{project}); err != nil {
		t.Fatal(err)
	}
	list, _ = s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false)
	if len(list) != 0 {
		t.Fatal("guest history resurrected deleted entry", list)
	}
	if _, err := os.Stat(history); err != nil {
		t.Fatal("CLI history was removed", err)
	}
}

func TestDiscoveryRunningStateUpdatesCatalogAndClearsMissingHistory(t *testing.T) {
	s, id, project := metadataFixture(t)
	home, _ := filepath.EvalSymlinks(t.TempDir())
	cliTestSSH(t, s, home, []string{"HOME=" + home, "PATH=/usr/bin:/bin:/usr/local/bin"})
	record, err := s.cliSessions.record(s.profile, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false); err != nil {
		t.Fatal(err)
	}
	// Keep the host bootstrap and SSH path; substitute the guest's observation.
	observation := filepath.Join(home, "observation.json")
	helper := filepath.Join(home, ".local/share/workbench/cli-channel-code", fmt.Sprintf("%x.py", sha256.Sum256([]byte(cliChannelHelper+cliResumePrompt+claudeAccountResumePrompt))))
	if err := os.MkdirAll(filepath.Dir(helper), 0700); err != nil {
		t.Fatal(err)
	}
	code := fmt.Sprintf("import json,sys\njson.load(sys.stdin)\nprint(open(%q).read())\n", observation)
	if err := os.WriteFile(helper, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	s.ready = true
	for _, phase := range []struct{ present, running, desktop bool }{{true, true, true}, {true, true, false}, {true, false, true}, {true, true, true}, {false, false, false}} {
		found := []terminalConversation{}
		if phase.present {
			found = append(found, terminalConversation{CLIID: record.CLIID, Workspace: guestPath(project), Config: filepath.Join(home, ".claude"), Running: phase.running, ClaudeDesktop: phase.desktop})
		}
		if err := writePrivate(observation, found); err != nil {
			t.Fatal(err)
		}
		if err := s.discoverTerminalConversations(context.Background(), []string{project}); err != nil {
			t.Fatal(err)
		}
		list, err := s.cliSessions.summaries(s.profile, map[string]bool{project: true}, false)
		if err != nil || len(list) != 1 || list[0].Running != phase.running || list[0].ClaudeDesktop != (phase.running && phase.desktop) {
			t.Fatalf("phase %+v: summaries = %+v, error = %v", phase, list, err)
		}
	}
}
