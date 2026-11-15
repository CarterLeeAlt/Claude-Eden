package eden

import _ "embed"

//go:embed claude-desktop-install.sh
var claudeDesktopInstall string

//go:embed claude-desktop-session.sh
var claudeDesktopSession string

//go:embed claude-desktop-open.sh
var claudeDesktopOpen string

//go:embed claude-desktop-start.sh
var claudeDesktopStart string

//go:embed guest-open.sh
var guestOpen string

//go:embed browser-login.mjs
var browserLogin string

func guestOpenScript() string {
	return "sudo tee /usr/local/lib/browser-login.mjs >/dev/null <<'WORKBENCH_BROWSER_LOGIN'\n" + browserLogin + "\nWORKBENCH_BROWSER_LOGIN\n" +
		"sudo tee /usr/local/bin/workbench-open >/dev/null <<'WORKBENCH_OPEN_URL'\n" + guestOpen + "\nWORKBENCH_OPEN_URL\nsudo chmod 0755 /usr/local/bin/workbench-open\nsudo ln -sf /usr/local/bin/workbench-open /usr/local/bin/xdg-open\n"
}

func ClaudeDesktopInstallScript() string {
	return ToolInstallPrelude() + claudeDesktopInstall + "\n" + ClaudeDesktopProvisionScript()
}

// Provisioning is independent of the browser and never restarts it. Desktop
// services are deliberately not enabled at boot: the user opens the app.
func ClaudeDesktopProvisionScript() string {
	return "set -euo pipefail\n" + clientRuntimeScript() + sharedClipboardScript() + nativeInputScript() + fileInputScript() + displayGovernorScript() +
		"sudo install -d -m 0755 /etc/workbench\n" +
		"sudo tee /usr/local/bin/workbench-claude-session >/dev/null <<'WORKBENCH_CLAUDE_SESSION'\n" + claudeDesktopSession + "\nWORKBENCH_CLAUDE_SESSION\n" +
		"sudo tee /usr/local/bin/workbench-claude-open >/dev/null <<'WORKBENCH_CLAUDE_OPEN'\n" + claudeDesktopOpen + "\nWORKBENCH_CLAUDE_OPEN\n" +
		"sudo tee /usr/local/bin/workbench-claude-start >/dev/null <<'WORKBENCH_CLAUDE_START'\n" + claudeDesktopStart + "\nWORKBENCH_CLAUDE_START\n" +
		"sudo chmod 0755 /usr/local/bin/workbench-claude-session /usr/local/bin/workbench-claude-open /usr/local/bin/workbench-claude-start\n" + guestOpenScript() + `
printf 'browser ALL=(developer) NOPASSWD: /usr/local/bin/workbench-claude-open *\n' | sudo tee /etc/sudoers.d/workbench-claude-desktop >/dev/null
sudo chmod 0440 /etc/sudoers.d/workbench-claude-desktop
sudo python3 - <<'WORKBENCH_DESKTOP_DOWNLOAD_CONFIG'
import json, os
from pathlib import Path
source = Path('/etc/workbench/browser-control.json')
if source.exists():
    with source.open() as config:
        workspace = json.load(config)['workspace']
    destination = Path('/etc/workbench/desktop-downloads.json')
    destination.write_text(json.dumps({'workspace': workspace}))
    os.chmod(destination, 0o644)
WORKBENCH_DESKTOP_DOWNLOAD_CONFIG
sudo tee /etc/workbench/desktop-openbox.xml >/dev/null <<'WORKBENCH_CLAUDE_WINDOWS'
<?xml version="1.0" encoding="UTF-8"?>
<openbox_config xmlns="http://openbox.org/3.4/rc">
<focus><focusNew>yes</focusNew><followMouse>no</followMouse></focus>
<desktops><number>1</number></desktops>
<applications><application type="normal"><decor>no</decor><maximized>yes</maximized></application></applications>
</openbox_config>
WORKBENCH_CLAUDE_WINDOWS
sudo tee /etc/systemd/system/workbench-claude-desktop.service >/dev/null <<'WORKBENCH_CLAUDE_UNIT'
[Unit]
Description=Claude Desktop session
[Service]
Type=notify
NotifyAccess=all
User=developer
WorkingDirectory=/home/developer
Environment=HOME=/home/developer
EnvironmentFile=/etc/workbench/environment
RuntimeDirectory=workbench-desktop
RuntimeDirectoryMode=0700
ExecStart=/usr/local/bin/workbench-log-run /usr/local/bin/workbench-claude-session
LogNamespace=workbench
KillMode=control-group
TimeoutStartSec=45
TimeoutStopSec=20
WORKBENCH_CLAUDE_UNIT
sudo systemctl daemon-reload
`
}
