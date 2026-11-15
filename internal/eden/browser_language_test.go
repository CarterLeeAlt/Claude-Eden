package eden

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBrowserLanguagePreferencesReplaceStaleSelection(t *testing.T) {
	interpreter := "python3"
	if runtime.GOOS == "windows" {
		interpreter = "python"
	}
	python, err := exec.LookPath(interpreter)
	if err != nil {
		t.Skip("guest Python interpreter unavailable")
	}
	start := strings.Index(guestStart, "import json, os\nfrom pathlib import Path\np = Path.home()")
	if start < 0 {
		t.Fatal("browser preference writer not found")
	}
	end := strings.Index(guestStart[start:], "\nPYCODE")
	script := strings.Replace(guestStart[start:start+end], "Path.home()", `Path(os.environ["FIXTURE"])`, 1)
	dir := t.TempDir()
	profile := filepath.Join(dir, "profile", "Default")
	if err := os.MkdirAll(profile, 0700); err != nil {
		t.Fatal(err)
	}
	prefs := filepath.Join(profile, "Preferences")
	if err := os.WriteFile(prefs, []byte(`{"intl":{"accept_languages":"en-US","selected_languages":"en-US"},"unrelated":{"keep":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"zh-TW", "fr-FR", "en-US"} {
		cmd := exec.Command(python, "-c", script)
		cmd.Env = append(os.Environ(), "FIXTURE="+dir, "WORKBENCH_LANGUAGE="+language)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		data, err := os.ReadFile(prefs)
		if err != nil {
			t.Fatal(err)
		}
		var settings struct {
			Intl      map[string]string `json:"intl"`
			Unrelated map[string]bool   `json:"unrelated"`
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}
		if settings.Intl["accept_languages"] != language || settings.Intl["selected_languages"] != language || !settings.Unrelated["keep"] {
			t.Fatalf("preference update lost language or unrelated settings: %s", data)
		}
	}
}
