package workbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/eden"
)

func TestPortableProjectPreferencesFollowDirectoryMove(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "original")
	profile := filepath.Join(app, "data", "profile")
	inside := filepath.Join(app, "data", "projects", "中文 project")
	outside := filepath.Join(root, "external-user-project")
	for _, dir := range []string{profile, inside, outside} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"portable.flag", "ClaudeEden.exe"} {
		if err := os.WriteFile(filepath.Join(app, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{profile: profile}
	preferences := Preferences{Workspace: inside, BrowserHomeURL: "https://example.test"}
	if err := savePreferences(profile, preferences); err != nil {
		t.Fatal(err)
	}
	saved := sharedProjects{Paths: []string{inside, outside}, Groups: []projectGroup{{Workspace: inside, Name: "Portable project", Folders: []string{inside, outside}}}}
	if err := s.saveSharedProjects(saved); err != nil {
		t.Fatal(err)
	}
	if saved.Paths[0] != inside || saved.Groups[0].Folders[0] != inside {
		t.Fatal("saving encoded paths mutated live grants")
	}
	for _, name := range []string{"workbench.json", "projects.json"} {
		b, err := os.ReadFile(filepath.Join(profile, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "original") || !strings.Contains(string(b), "@data/") {
			t.Fatalf("%s kept the old portable root: %s", name, b)
		}
	}
	moved := filepath.Join(root, "moved 位置")
	if err := os.Rename(app, moved); err != nil {
		t.Fatal(err)
	}
	profile = filepath.Join(moved, "data", "profile")
	s.profile = profile
	loaded, err := s.readSharedProjects()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(moved, "data", "projects", "中文 project")
	if len(loaded.Paths) != 2 || loaded.Paths[0] != want || loaded.Paths[1] != outside || loaded.Groups[0].Workspace != want || loaded.Groups[0].Folders[0] != want {
		t.Fatalf("moved grants incorrect: %+v", loaded)
	}
	b, err := os.ReadFile(filepath.Join(profile, "workbench.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &preferences); err != nil {
		t.Fatal(err)
	}
	resolved, err := eden.ResolvePortablePath(profile, preferences.Workspace)
	if err != nil || resolved != want {
		t.Fatalf("moved active project=%q: %v", resolved, err)
	}
	if _, err := os.Stat(app); !os.IsNotExist(err) {
		t.Fatalf("old directory was recreated: %v", err)
	}
}
