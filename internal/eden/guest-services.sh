# Publish browser assets while the web endpoint is stopped, so clients cannot
# mix old and new noVNC modules during an upgrade.
if systemctl is-active --quiet workbench-webvnc; then
  sudo systemctl stop workbench-webvnc
fi
if ! command -v Xtigervnc >/dev/null || ! command -v openbox >/dev/null || ! command -v xrandr >/dev/null || ! command -v xdotool >/dev/null || ! command -v ibus-daemon >/dev/null || ! /usr/bin/python3 -c 'import gi; gi.require_version("IBus", "1.0"); gi.require_version("Gtk", "3.0")' 2>/dev/null; then
  sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get install -y --no-install-recommends tigervnc-standalone-server tigervnc-tools openbox x11-xserver-utils xdotool libgtk-3-0 ibus ibus-gtk3 gir1.2-ibus-1.0 gir1.2-gtk-3.0 python3-gi python3-xlib xsettingsd
fi
workbench_novnc_root=/usr/share/novnc
sudo install -d -m 0755 /usr/local/share
sudo ln -sfn "$workbench_novnc_root" /usr/local/share/workbench-webvnc
sudo install -d -m 0755 /etc/workbench /usr/local/lib
# New website passwords belong to the host vault. Keep existing browser data.
workbench_browser_policy_dir=/etc/chromium/policies/managed
sudo install -d -m 0755 "$workbench_browser_policy_dir"
printf '%s\n' '{"PasswordManagerEnabled":false}' | sudo tee "$workbench_browser_policy_dir/workbench-password-vault.json" >/dev/null
sudo chmod 0644 "$workbench_browser_policy_dir/workbench-password-vault.json"
# Resolve Tailscale application links in the workbench browser's user session.
if ! command -v xdg-mime >/dev/null; then
  sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get install -y --no-install-recommends xdg-utils </dev/null
fi
sudo install -d -m 0755 /usr/local/share/applications
printf '%s\n' '[Desktop Entry]' 'Type=Application' 'Name=Tailscale' 'NoDisplay=true' 'Exec=/usr/local/bin/workbench-open %u' 'MimeType=x-scheme-handler/tailscale;' | sudo tee /usr/local/share/applications/tailnet-login.desktop >/dev/null
sudo -Hu browser xdg-mime default tailnet-login.desktop x-scheme-handler/tailscale
sudo tee /usr/local/bin/workbench-editor-start >/dev/null <<'SCRIPT'
#!/bin/sh
set -eu
workspace=$(cat /home/developer/.config/workbench/editor-workspace)
exec /usr/bin/code-server --bind-addr 127.0.0.1:18080 --config /home/developer/.config/workbench/code-server.yaml --disable-telemetry --disable-update-check --disable-workspace-trust --disable-proxy "$workspace"
SCRIPT
sudo chmod 0755 /usr/local/bin/workbench-editor-start
sudo tee /etc/systemd/system/workbench-editor.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench file editor
[Service]
User=developer
WorkingDirectory=/home/developer
Environment=HOME=/home/developer
EnvironmentFile=/etc/workbench/environment
ExecStart=/usr/local/bin/workbench-log-run /usr/local/bin/workbench-editor-start
LogNamespace=workbench
Restart=on-failure
RestartSec=2
TimeoutStopSec=15
UNIT
printf 'browser ALL=(root) NOPASSWD: /usr/bin/systemctl restart workbench-editor.service\n' | sudo tee /etc/sudoers.d/workbench-editor >/dev/null
sudo chmod 0440 /etc/sudoers.d/workbench-editor
sudo tee /etc/profile.d/workbench.sh >/dev/null <<'SCRIPT'
if [ -f /etc/workbench/environment ]; then
  set -a
  . /etc/workbench/environment
  set +a
fi
unset HTTP_PROXY HTTPS_PROXY ALL_PROXY FTP_PROXY NO_PROXY http_proxy https_proxy all_proxy ftp_proxy no_proxy
export BROWSER=/usr/local/bin/workbench-open
SCRIPT
if ! grep -q '/etc/profile.d/workbench.sh' /etc/zsh/zshenv; then
  echo '. /etc/profile.d/workbench.sh' | sudo tee -a /etc/zsh/zshenv >/dev/null
fi
sudo tee /etc/systemd/system/workbench-display.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench virtual browser display
Wants=workbench-display-governor.service
[Service]
User=browser
ExecStartPre=/usr/bin/rm -f /home/browser/.vnc/display.sock
ExecStart=/usr/local/bin/workbench-log-run /usr/bin/Xtigervnc :77 -geometry 2880x1800 -depth 24 -rfbport 15900 -localhost -nolisten tcp -SecurityTypes VncAuth -PasswordFile /home/browser/.vnc/passwd -AlwaysShared
LogNamespace=workbench
Restart=on-failure
UNIT
sudo tee /etc/workbench/openbox.xml >/dev/null <<'WINDOW_CONFIG'
<?xml version="1.0" encoding="UTF-8"?>
<openbox_config xmlns="http://openbox.org/3.4/rc">
<focus><focusNew>yes</focusNew><followMouse>no</followMouse></focus>
<desktops><number>1</number></desktops>
<keyboard></keyboard><mouse></mouse>
<applications><application type="normal"><decor>no</decor><maximized>yes</maximized></application></applications>
</openbox_config>
WINDOW_CONFIG
sudo tee /etc/systemd/system/workbench-window-manager.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench window geometry and focus (no desktop shell)
Requires=workbench-display.service
After=workbench-display.service
[Service]
User=browser
Environment=DISPLAY=:77
ExecStart=/usr/local/bin/workbench-log-run /usr/bin/openbox --config-file /etc/workbench/openbox.xml
LogNamespace=workbench
Restart=on-failure
UNIT
sudo tee /etc/systemd/system/workbench-browser.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench persistent browser
Requires=workbench-display.service workbench-window-manager.service
After=workbench-display.service workbench-window-manager.service
[Service]
User=browser
Environment=DISPLAY=:77
EnvironmentFile=/etc/workbench/environment
ExecStart=/usr/local/bin/workbench-log-run /usr/local/bin/workbench-input-session /usr/bin/node /usr/local/lib/workbench-browser-agent.mjs
LogNamespace=workbench
# Let the controller close Chromium and flush its tabs and profile before
# systemd terminates any remaining child processes.
KillMode=mixed
Restart=on-failure
RestartSec=2
UNIT
# Starts, restarts and stops with the display it paces. The display only
# wants it: a guest without python3-xlib keeps its full refresh rate.
sudo tee /etc/systemd/system/workbench-display-governor.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench browser display pacing
PartOf=workbench-display.service
After=workbench-display.service
[Service]
User=browser
Environment=DISPLAY=:77
# The browser controller reports here the scrolling it delivers to a page.
Environment=WORKBENCH_DISPLAY_ACTIVITY=/home/browser/.cache/workbench-display/activity.sock
ExecStart=/usr/local/bin/workbench-log-run /usr/local/bin/workbench-display-governor
LogNamespace=workbench
Restart=on-failure
RestartSec=5
UNIT
sudo tee /etc/systemd/system/workbench-webvnc.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench browser web access
Requires=workbench-display.service
After=workbench-display.service
[Service]
User=browser
ExecStart=/usr/local/bin/workbench-log-run /usr/bin/websockify --web=/usr/local/share/workbench-webvnc 127.0.0.1:16080 127.0.0.1:15900
LogNamespace=workbench
Restart=on-failure
UNIT
sudo systemctl daemon-reload

if ! grep -q 'WORKBENCH_PROMPT' /home/developer/.bashrc; then
  sudo tee -a /home/developer/.bashrc >/dev/null <<'PROMPT'
# WORKBENCH_PROMPT
if [ "${WORKBENCH_SESSION:-}" = 1 ]; then
  PS1='\[\e[32m\]developer\[\e[0m\]:\[\e[34m\]\W\[\e[0m\]\$ '
fi
PROMPT
fi
