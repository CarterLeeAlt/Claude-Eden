package eden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentIdentityIsIndependentOfProjectSelection(t *testing.T) {
	profile := t.TempDir()
	transfer, _ := filepath.EvalSymlinks(t.TempDir())
	prefs, _ := json.Marshal(map[string]string{"workspace": transfer})
	if err := os.WriteFile(filepath.Join(profile, "workbench.json"), prefs, 0600); err != nil {
		t.Fatal(err)
	}
	if EnvironmentDir(profile) != DefaultEnvironmentDir(profile) {
		t.Fatal("selected project changed the default transfer directory")
	}
	saved, _ := json.Marshal(map[string]string{"directory": transfer})
	if err := os.WriteFile(filepath.Join(profile, "environment.json"), saved, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "workbench.json"), []byte(`{"workspace":"/different-project"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if EnvironmentDir(profile) != transfer {
		t.Fatal("changing project changed the environment VM")
	}
}
