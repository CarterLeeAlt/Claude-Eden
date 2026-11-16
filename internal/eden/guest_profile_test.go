package eden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/guestprofile"
)

func TestGuestProfileRendersOwnedPayloadsAndReloadsByValue(t *testing.T) {
	dir := t.TempDir()
	profile, err := guestprofile.Create(dir, guestprofile.Options{Mode: "manual", Hostname: "X", Username: "alex", BrowserUsername: "web", WorkHome: "/home/projects", BrowserHome: "/home/browsing", AudioName: "speakers", AudioDescription: "Desktop (USB)", GraphicsBackend: "opengl"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	b, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	first, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(path)
	if err != nil || first != second || first.GuestProfile != *profile {
		t.Fatal("reload changed comparable configuration", err)
	}
	for name, script := range map[string]string{"setup": first.SetupScript(), "start": first.StartScript(), "desktop": profile.Linux(ClaudeDesktopInstallScript())} {
		for _, old := range []string{"/home/developer/", "/home/browser/", "User=developer\n", "User=browser\n", "-u developer ", "-u browser ", "browser ALL=", "ALL=(developer)", `"$(id -un)" = developer`, `"$(id -un)" = browser`} {
			if strings.Contains(script, old) {
				t.Errorf("%s retained %q", name, old)
			}
		}
	}
	start := first.StartScript()
	for _, expected := range []string{"User=alex", "User=web", "/home/projects/", "/home/browsing/", "--use-angle=gl", "sink_name=speakers", "workbench-browser.service", "browser-panes-" + profile.ExtensionSuffix, "/usr/local/lib/device-observation.mjs"} {
		if !strings.Contains(start, expected) {
			t.Errorf("startup missing %q", expected)
		}
	}
}
