package eden

import (
	"regexp"
	"strings"
	"testing"

	agentvm "github.com/sylvinus/agent-vm"
)

// Inspect the assembled payloads: checking individual source files misses
// command names, environment variables and paths added by the host wrappers.
func TestGuestArtifactsUseNeutralNames(t *testing.T) {
	config := Config{Timezone: "Etc/UTC", Locale: "en_US.UTF-8", BrowserLanguage: "en-US"}
	for name, script := range map[string]string{
		"installation": GuestSetup,
		"base-setup":   agentvm.SetupScript,
		"startup":      config.StartScript(),
		"tools-update": ToolsUpdateScript(),
		"desktop":      ClaudeDesktopInstallScript(),
	} {
		t.Run(name, func(t *testing.T) {
			// These are forbidden product identifiers, not arbitrary user input.
			// "credentials" contains "eden", so the new host brand needs token
			// boundaries rather than a substring check.
			forbidden := regexp.MustCompile(`(?i)cage|claude[-_ ]eden|\beden[_-]|dev\.eden|Eden\.Guest`)
			for index, line := range strings.Split(script, "\n") {
				if token := forbidden.FindString(line); token != "" {
					t.Errorf("guest payload line %d contains product identifier %q", index+1, token)
				}
			}
		})
	}
}

func TestGuestArtifactsOnlyProvisionLocalDebianComponents(t *testing.T) {
	for name, script := range map[string]string{
		"installation": GuestSetup,
		"base-setup":   agentvm.SetupScript,
		"startup":      DefaultConfig().StartScript(),
		"tools-update": ToolsUpdateScript(),
		"desktop":      ClaudeDesktopInstallScript(),
	} {
		t.Run(name, func(t *testing.T) {
			for _, removed := range []string{"/phone-browser/", "/phone-desktop/", "workbench-phone-", "WORKBENCH_PHONE_", "mobile_settings_fd", "verify_mobile", "snapcraft", "/var/snap/", "/snap/bin/", "workbench_is_ubuntu", "WORKBENCH_UBUNTU_"} {
				if strings.Contains(script, removed) {
					t.Errorf("assembled guest payload retains removed component %q", removed)
				}
			}
		})
	}
}
