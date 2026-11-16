package eden

import (
	"al.essio.dev/pkg/shellescape"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/sylvinus/agent-vm/internal/version"
)

//go:embed guest-setup.sh
var guestDependencies string

//go:embed tailnet-client.mjs
var tailnetClient string

func tailnetClientScript() string {
	return "sudo tee /usr/local/lib/tailnet-client.mjs >/dev/null <<'WORKBENCH_TAILNET_CLIENT'\n" + tailnetClient + "\nWORKBENCH_TAILNET_CLIENT\n"
}

//go:embed debian-browser.sh
var debianBrowser string

//go:embed guest-services.sh
var guestServices string

//go:embed guest-wait-services.py
var guestWaitServices string

//go:embed claude-account.py
var claudeAccountHelper string

//go:embed client_runtime.py
var ClientRuntimeHelper string

func clientRuntimeScript() string {
	return "sudo tee /usr/local/bin/client_runtime.py >/dev/null <<'CLIENT_RUNTIME'\n" + ClientRuntimeHelper + "\nCLIENT_RUNTIME\nsudo chmod 0644 /usr/local/bin/client_runtime.py\n"
}

func claudeAccountScript() string {
	return clientRuntimeScript() +
		"sudo tee /usr/local/bin/workbench-claude-account >/dev/null <<'WORKBENCH_CLAUDE_ACCOUNT'\n" + claudeAccountHelper + "\nWORKBENCH_CLAUDE_ACCOUNT\nsudo chmod 0755 /usr/local/bin/workbench-claude-account\n" +
		`sudo install -d -m 0755 /usr/local/lib/workbench-claude-bin
sudo tee /usr/local/lib/workbench-claude-bin/claude >/dev/null <<'WORKBENCH_CLAUDE_SHIM'
#!/bin/sh
exec /usr/local/bin/workbench-claude-account run "$WORKBENCH_CLAUDE_ACCOUNT_ID" "$@"
WORKBENCH_CLAUDE_SHIM
sudo chmod 0755 /usr/local/lib/workbench-claude-bin/claude
`
}

//go:embed guest-audio.sh
var guestAudio string

//go:embed guest-memory.sh
var guestMemory string

//go:embed novnc-patch.py
var novncPatch string

//go:embed clipboard.js
var clipboardJS string

//go:embed browser-vault.mjs
var browserVault string

//go:embed browser-agent.mjs
var browserAgent string

//go:embed browser-editor.mjs
var browserEditor string

//go:embed browser-visibility.mjs
var browserVisibility string

//go:embed browser-wheel.mjs
var browserWheel string

//go:embed display-density.mjs
var displayDensity string

//go:embed display-governor.py
var displayGovernor string

func displayGovernorScript() string {
	return "sudo tee /usr/local/bin/workbench-display-governor >/dev/null <<'WORKBENCH_DISPLAY_GOVERNOR'\n" + displayGovernor + "\nWORKBENCH_DISPLAY_GOVERNOR\nsudo chmod 0755 /usr/local/bin/workbench-display-governor\n"
}

//go:embed browser-private.mjs
var browserPrivate string

//go:embed browser-cookies.mjs
var browserCookies string

//go:embed browser-extensions.mjs
var browserExtensions string

//go:embed browser-extension-archive.py
var browserExtensionArchive string

//go:embed browser-clipboard.mjs
var browserClipboard string

//go:embed split-extension/manifest.json
var splitManifest string

//go:embed split-extension/control.html
var splitPage string

//go:embed split-extension/settings.html
var browserSettingsPage string

//go:embed split-extension/control.js
var splitControl string

func splitExtensionScript() string {
	return "split_directory=/usr/local/lib/workbench-split\nsudo install -d -m 0755 \"$split_directory\"\n" +
		"sudo tee \"$split_directory\"/manifest.json >/dev/null <<'WORKBENCH_MANIFEST'\n" + splitManifest + "\nWORKBENCH_MANIFEST\n" +
		"sudo tee \"$split_directory\"/control.html >/dev/null <<'WORKBENCH_HTML'\n" + splitPage + "\nWORKBENCH_HTML\n" +
		"sudo tee \"$split_directory\"/settings.html >/dev/null <<'WORKBENCH_SETTINGS_HTML'\n" + browserSettingsPage + "\nWORKBENCH_SETTINGS_HTML\n" +
		"sudo tee \"$split_directory\"/control.js >/dev/null <<'WORKBENCH_CONTROL'\n" + splitControl + "\nWORKBENCH_CONTROL\n"
}

func browserClipboardScript() string {
	return sharedClipboardScript() + "sudo tee /usr/local/lib/browser-clipboard.mjs >/dev/null <<'WORKBENCH_BROWSER_CLIPBOARD'\n" + browserClipboard + "\nWORKBENCH_BROWSER_CLIPBOARD\n"
}

//go:embed browser-audit.mjs
var browserAudit string

//go:embed device-observation.mjs
var deviceObservation string

func deviceObservationScript() string {
	return "sudo tee /usr/local/lib/device-observation.mjs >/dev/null <<'WORKBENCH_DEVICE_OBSERVATION'\n" + deviceObservation + "\nWORKBENCH_DEVICE_OBSERVATION\n"
}

//go:embed browser-downloads.mjs
var browserDownloads string

//go:embed browser-uploads.mjs
var browserUploads string

//go:embed save-download.py
var saveDownload string

//go:embed image-input.py
var imageInput string

//go:embed desktop-files.py
var desktopFiles string

// DesktopFileReceiver shares the same bounded attachment writer with native
// guests. Its --save-file mode uses only Python's standard library.
func DesktopFileReceiver() string { return desktopFiles }

//go:embed desktop-downloads.py
var desktopDownloads string

//go:embed native-ime.py
var nativeIME string

//go:embed native-ime-session.sh
var nativeIMESession string

//go:embed native-input.mjs
var nativeInputClient string

func nativeInputScript() string {
	return "\nsudo tee /usr/local/bin/workbench-native-input >/dev/null <<'WORKBENCH_NATIVE_INPUT'\n" + nativeIME + "\nWORKBENCH_NATIVE_INPUT\nsudo chmod 0755 /usr/local/bin/workbench-native-input\n" +
		"sudo tee /usr/local/bin/workbench-input-session >/dev/null <<'WORKBENCH_INPUT_SESSION'\n" + nativeIMESession + "\nWORKBENCH_INPUT_SESSION\nsudo chmod 0755 /usr/local/bin/workbench-input-session\n" +
		"sudo tee /usr/local/lib/native-input.mjs >/dev/null <<'WORKBENCH_INPUT_CLIENT'\n" + nativeInputClient + "\nWORKBENCH_INPUT_CLIENT\n" +
		"printf 'browser ALL=(developer) NOPASSWD: /usr/local/bin/workbench-native-input --request, /usr/local/bin/workbench-native-input --relay\\n' | sudo tee /etc/sudoers.d/workbench-input >/dev/null\nsudo chmod 0440 /etc/sudoers.d/workbench-input\n"
}

func fileInputScript() string {
	return "\nsudo tee /usr/local/bin/workbench-desktop-files >/dev/null <<'WORKBENCH_DESKTOP_FILES'\n" + desktopFiles + "\nWORKBENCH_DESKTOP_FILES\nsudo chmod 0755 /usr/local/bin/workbench-desktop-files\n" +
		"sudo tee /usr/local/lib/workbench-desktop-downloads.py >/dev/null <<'WORKBENCH_DESKTOP_DOWNLOADS'\n" + desktopDownloads + "\nWORKBENCH_DESKTOP_DOWNLOADS\n" +
		"sudo tee /usr/local/bin/workbench-save-download >/dev/null <<'WORKBENCH_DESKTOP_DOWNLOAD_COPY'\n" + saveDownload + "\nWORKBENCH_DESKTOP_DOWNLOAD_COPY\nsudo chmod 0755 /usr/local/bin/workbench-save-download\n"
}

func imageInputScript() string {
	return nativeInputScript() + fileInputScript() + "\nsudo tee /usr/local/bin/workbench-image-input >/dev/null <<'WORKBENCH_IMAGE_INPUT'\n" + imageInput + "\nWORKBENCH_IMAGE_INPUT\nsudo chmod 0755 /usr/local/bin/workbench-image-input\n"
}

func browserServices() string {
	return deviceObservationScript() + "sudo tee /usr/local/lib/workbench-wait-services.py >/dev/null <<'WORKBENCH_WAIT_SERVICES'\n" + guestWaitServices + "\nWORKBENCH_WAIT_SERVICES\n" + toolInstallFunctions() + "\n" + tailnetClientScript() +
		"sudo tee /usr/local/lib/browser-visibility.mjs >/dev/null <<'WORKBENCH_BROWSER_VISIBILITY'\n" + browserVisibility + "\nWORKBENCH_BROWSER_VISIBILITY\nsudo tee /usr/local/lib/browser-wheel.mjs >/dev/null <<'WORKBENCH_BROWSER_WHEEL'\n" + browserWheel + "\nWORKBENCH_BROWSER_WHEEL\nsudo tee /usr/local/lib/display-density.mjs >/dev/null <<'WORKBENCH_DISPLAY_DENSITY'\n" + displayDensity + "\nWORKBENCH_DISPLAY_DENSITY\n" + displayGovernorScript() +
		"\nsudo tee /usr/local/lib/browser-extensions.mjs >/dev/null <<'WORKBENCH_BROWSER_EXTENSIONS'\n" + browserExtensions + "\nWORKBENCH_BROWSER_EXTENSIONS\n" + "\nsudo tee /usr/local/lib/browser-extension-archive.py >/dev/null <<'WORKBENCH_EXTENSION_ARCHIVE'\n" + browserExtensionArchive + "\nWORKBENCH_EXTENSION_ARCHIVE\n" +
		"\nsudo tee /usr/local/lib/browser-private.mjs >/dev/null <<'WORKBENCH_BROWSER_PRIVATE'\n" + browserPrivate + "\nWORKBENCH_BROWSER_PRIVATE\n" + "\nsudo tee /usr/local/lib/browser-cookies.mjs >/dev/null <<'WORKBENCH_BROWSER_COOKIES'\n" + browserCookies + "\nWORKBENCH_BROWSER_COOKIES\n" + "\nsudo tee /usr/local/lib/browser-vault.mjs >/dev/null <<'WORKBENCH_BROWSER_VAULT'\n" + browserVault + "\nWORKBENCH_BROWSER_VAULT\n" +
		guestMemory + "\n" + guestDiagnostics() + guestAudio + "\n" + guestOpenScript() + splitExtensionScript() + browserClipboardScript() + imageInputScript() +
		"\nsudo tee /usr/local/lib/browser-uploads.mjs >/dev/null <<'WORKBENCH_BROWSER_UPLOADS'\n" + browserUploads + "\nWORKBENCH_BROWSER_UPLOADS\n" + guestServices + "\nsudo tee /usr/local/lib/browser-downloads.mjs >/dev/null <<'WORKBENCH_BROWSER_DOWNLOADS'\n" + browserDownloads + "\nWORKBENCH_BROWSER_DOWNLOADS\n" +
		"\nsudo tee /usr/local/lib/browser-editor.mjs >/dev/null <<'WORKBENCH_BROWSER_EDITOR'\n" + browserEditor + "\nWORKBENCH_BROWSER_EDITOR\n" +
		"\nsudo tee /usr/local/lib/browser-audit.mjs >/dev/null <<'WORKBENCH_BROWSER_AUDIT'\n" + browserAudit + "\nWORKBENCH_BROWSER_AUDIT\n" + "\nsudo tee /usr/local/lib/workbench-browser-agent.mjs >/dev/null <<'WORKBENCH_BROWSER_AGENT'\n" + browserAgent + "\nWORKBENCH_BROWSER_AGENT\n" +
		"sudo tee /usr/local/bin/workbench-save-download >/dev/null <<'WORKBENCH_DOWNLOAD_HELPER'\n" + saveDownload + "\nWORKBENCH_DOWNLOAD_HELPER\nsudo chmod 0755 /usr/local/bin/workbench-save-download\nprintf 'browser ALL=(root) NOPASSWD: /usr/local/bin/workbench-save-download *\\n' | sudo tee /etc/sudoers.d/workbench-download >/dev/null\nsudo chmod 0440 /etc/sudoers.d/workbench-download\n" +
		"\nsudo python3 - \"$workbench_novnc_root\" <<'WORKBENCH_NOVNC_PATCH'\n" + novncPatch + "\nWORKBENCH_NOVNC_PATCH\n" + "\nsudo tee \"$workbench_novnc_root/workbench-clipboard.js\" >/dev/null <<'WORKBENCH_CLIPBOARD_JS'\n" + clipboardJS + "\nWORKBENCH_CLIPBOARD_JS\n" + `sudo python3 - "$workbench_novnc_root" <<'PYCODE'
from pathlib import Path
import re
import sys
p = Path(sys.argv[1]) / "vnc.html"
s = p.read_text()
tag = '<script type="module" src="workbench-clipboard.js?revision=8"></script>'
pattern = r'<script type="module" src="workbench-clipboard\.js(?:\?revision=\d+)?"></script>'
if re.search(pattern, s):
    s = re.sub(pattern, lambda _: tag, s)
else:
    s = s.replace("</body>", tag + "\n</body>")
p.write_text(s)
PYCODE
`
}

var GuestSetup = APTBootstrapScript + "\n" + guestDependencies + "\n" + browserServices()

//go:embed editor-theme.json
var editorTheme string

//go:embed guest-start.sh
var guestStart string

//go:embed guest-env.sh
var guestEnv string

//go:embed tool-versions.env
var toolVersions string

//go:embed tool-download.py
var toolDownload string

//go:embed guest-tools.sh
var guestTools string

//go:embed debian-mirrors.sh
var DebianMirrorsScript string

//go:embed apt-bootstrap.sh
var aptBootstrap string

// APTBootstrapScript preserves the temporary maintenance proxy through sudo.
var APTBootstrapScript = "set -e\nsudo env APT_CONFIG=\"${WORKBENCH_APT_CONFIG:-/dev/null}\" sh -se <<'WORKBENCH_APT_BOOTSTRAP'\n" + aptBootstrap + "\nWORKBENCH_APT_BOOTSTRAP\n"

// DebianPrepareScript runs before Lima's own dependency installation as well
// as environment setup. Its version check also covers component updates.
var DebianPrepareScript = "#!/bin/sh\n" + APTBootstrapScript + "\n" + DebianMirrorsScript

//go:embed chromium-download.py
var chromiumDownload string

func toolInstallFunctions() string {
	return toolVersions + `
workbench_curl_download() {
  python3 - "$@" <<'WORKBENCH_DOWNLOAD_IDLE'
` + toolDownload + `
WORKBENCH_DOWNLOAD_IDLE
}
` + guestTools + "\n" + debianBrowser
}

func ToolInstallPrelude() string {
	return toolInstallFunctions() + `
workbench_cache_chromium() {
  local status=0
  sudo env APT_CONFIG="${WORKBENCH_APT_CONFIG:-}" WORKBENCH_SETUP_PROGRESS="${WORKBENCH_SETUP_PROGRESS:-}" /usr/bin/python3 -I - "$@" <<'WORKBENCH_CHROMIUM_DOWNLOAD' || status=$?
` + chromiumDownload + `
WORKBENCH_CHROMIUM_DOWNLOAD
  if (( status >= 128 )); then return "$status"; fi
  if (( status != 0 )); then echo 'Chromium mirror prefetch unavailable; continuing with APT.' >&2; fi
}
`
}

func RequiredToolVersions() map[string]string {
	result := map[string]string{"chromium": "Debian 13 security repository"}
	for _, line := range strings.Split(toolVersions, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "WORKBENCH_CODE_SERVER_VERSION":
			result["code_server"] = value
		case "WORKBENCH_MISE_VERSION":
			result["mise"] = value
		case "WORKBENCH_GH_VERSION":
			result["gh"] = value
		}
	}
	result["claude"] = "官方最新版（下载时解析）"
	return result
}

func ToolsUpdateScript() string {
	return "set -euo pipefail\nexport PATH=\"$HOME/.local/bin:$PATH\"\n" + `
sudo env DEBIAN_FRONTEND=noninteractive dpkg --configure -a </dev/null
` + APTBootstrapScript + ToolInstallPrelude() + `
sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get update </dev/null
workbench_install_chromium
sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get install -y --no-install-recommends python3-xlib </dev/null || echo 'python3-xlib is unavailable; displays keep their full refresh rate.' >&2
sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get install -y --no-install-recommends xsettingsd </dev/null || echo 'xsettingsd is unavailable; displays keep their configured density.' >&2
workbench_install_tool mise
workbench_install_tool gh
workbench_install_tool code-server
workbench_install_tool claude
printf 'Workbench tool packages are ready; applying browser and editor components.\n'
`
}

func (c Config) EnvironmentScript() string {
	return fmt.Sprintf("export WORKBENCH_TZ=%s\nexport WORKBENCH_LOCALE=%s\nexport WORKBENCH_LANGUAGE=%s\n", shellescape.Quote(c.Timezone), shellescape.Quote(c.Locale), shellescape.Quote(c.BrowserLanguage)) + guestEnv
}

func (c Config) StartScript() string {
	body := c.EnvironmentScript() + "\n" + componentStartScript()
	return c.GuestProfile.Linux(markGuestStart(body))
}

func (c Config) SetupScript() string { return c.GuestProfile.Linux(GuestSetup) }

func componentStartScript() string {
	// Drain Chromium while its editor and logging dependencies are still alive,
	// before replacing scripts or restarting the editor and audio services.
	return `for service in workbench-browser workbench-editor; do
  if [ -f "/etc/systemd/system/$service.service" ]; then
    sudo systemctl stop "$service.service"
  fi
done
` + APTBootstrapScript + "\n" + claudeAccountScript() + browserServices() + "\nsudo tee /etc/workbench/editor-theme.json >/dev/null <<'WORKBENCH_EDITOR_THEME'\n" + editorTheme + "\nWORKBENCH_EDITOR_THEME\n" + guestStart
}

func markGuestStart(body string) string {
	manifest, _ := json.Marshal(RequiredGuestManifest())
	return "set -euo pipefail\nsudo rm -f /etc/workbench/components.json\n(\n" + body + "\n)\nsudo python3 - <<'WORKBENCH_COMPONENT_MANIFEST'\nfrom pathlib import Path\nimport os,tempfile\np=Path('/etc/workbench/components.json')\nfd,name=tempfile.mkstemp(prefix='.components-',dir=p.parent)\nwith os.fdopen(fd,'w') as f:\n f.write(" + fmt.Sprintf("%q", string(manifest)) + ")\n f.flush();os.fsync(f.fileno())\nos.chmod(name,0o644)\nos.replace(name,p)\nWORKBENCH_COMPONENT_MANIFEST\n"
}

type GuestManifest struct {
	Protocol   int    `json:"protocol"`
	Revision   string `json:"revision"`
	AppVersion string `json:"app_version"`
}

var guestRevision = sync.OnceValue(func() string {
	payload := strings.Join([]string{"workbench-guest-protocol-1", guestDependencies, guestEnv, componentStartScript(), ToolInstallPrelude()}, "\x00")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
})

func RequiredGuestManifest() GuestManifest {
	return GuestManifest{Protocol: 1, Revision: guestRevision(), AppVersion: version.Version}
}

func (m GuestManifest) Current() bool {
	required := RequiredGuestManifest()
	return m.Protocol == required.Protocol && m.Revision == required.Revision
}

func guestDiagnostics() string {
	return "sudo tee /usr/local/bin/workbench-log-run >/dev/null <<'WORKBENCH_LOG_RUN'\n" + diagnostics.GuestRunner() + "\nWORKBENCH_LOG_RUN\nsudo chmod 0755 /usr/local/bin/workbench-log-run\n" + `sudo install -d -m 0755 /etc/systemd/journald@workbench.conf.d
sudo tee /etc/systemd/journald@workbench.conf.d/limits.conf >/dev/null <<'WORKBENCH_JOURNAL'
[Journal]
Storage=persistent
SystemMaxUse=10M
SystemMaxFileSize=1M
RuntimeMaxUse=10M
RuntimeMaxFileSize=1M
MaxRetentionSec=14day
MaxFileSec=1day
RateLimitIntervalSec=30s
RateLimitBurst=200
WORKBENCH_JOURNAL
sudo systemctl try-restart systemd-journald@workbench.service
`
}
