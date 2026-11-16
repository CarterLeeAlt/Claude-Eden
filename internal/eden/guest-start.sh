#!/usr/bin/env bash
set -euo pipefail
if [[ ! -x /usr/bin/chromium ]]; then
  echo 'Missing chromium; run Workbench setup first.' >&2
  exit 1
fi
for app in code-server Xtigervnc openbox websockify; do
  command -v "$app" >/dev/null || { echo "Missing $app; run Workbench setup first." >&2; exit 1; }
done
# English text keeps its requested fonts. For missing Han glyphs, prefer the
# Simplified Chinese face of the installed Noto collection.
sudo tee /etc/fonts/conf.d/71-english-cjk-fallback.conf >/dev/null <<'FONTCONFIG'
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <match target="pattern">
    <test name="lang" qual="first" compare="contains"><string>en</string></test>
    <test name="family"><string>sans-serif</string></test>
    <edit name="family" mode="append_last" binding="weak"><string>Noto Sans CJK SC</string></edit>
  </match>
  <match target="pattern">
    <test name="lang" qual="first" compare="contains"><string>en</string></test>
    <test name="family"><string>serif</string></test>
    <edit name="family" mode="append_last" binding="weak"><string>Noto Serif CJK SC</string></edit>
  </match>
  <match target="pattern">
    <test name="lang" qual="first" compare="contains"><string>en</string></test>
    <test name="family"><string>monospace</string></test>
    <edit name="family" mode="append_last" binding="weak"><string>Noto Sans Mono CJK SC</string></edit>
  </match>
</fontconfig>
FONTCONFIG
mkdir -p "$HOME/.config/workbench"
chmod 700 "$HOME/.config/workbench"
if [ ! -s "$HOME/.config/workbench/code-server.yaml" ]; then
  (umask 077; printf 'auth: password\npassword: %s\ncert: false\n' "$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')" > "$HOME/.config/workbench/code-server.yaml")
fi
if [ ! -s "$HOME/.config/workbench/browser-password" ]; then
  (umask 077; od -An -N4 -tx1 /dev/urandom | tr -d ' \n' > "$HOME/.config/workbench/browser-password")
fi
sudo install -d -o browser -g browser -m 0700 /home/browser/.vnc
cat "$HOME/.config/workbench/browser-password" | sudo -u browser sh -c 'umask 077; tigervncpasswd -f > "$HOME/.vnc/passwd"'
# Publish per-VM control credentials for the browser process and SSH owner only.
sudo env WORKBENCH_DISPLAY_SCALE="${WORKBENCH_DISPLAY_SCALE:-2}" WORKBENCH_WORKSPACE="$PWD" WORKBENCH_USER_HOME="$HOME" python3 - <<'PYCONTROL'
import json, os, secrets, grp
from pathlib import Path
p=Path('/etc/workbench/browser-control.json')
d=json.loads(p.read_text()) if p.exists() else {'token':secrets.token_hex(32)}
scale = 1.5 if os.environ['WORKBENCH_DISPLAY_SCALE'] == '1.5' else 2
d['display_scale'] = scale
# With a settings manager each viewer chooses the density of its own screen.
d['dynamic_scale'] = os.path.exists('/usr/bin/xsettingsd')
scale_file = Path('/etc/workbench/display-scale')
scale_file.write_text(str(scale) + '\n')
scale_file.chmod(0o644)
d['workspace']=os.environ['WORKBENCH_WORKSPACE']
yaml=Path(os.environ['WORKBENCH_USER_HOME']) / '.config/workbench/code-server.yaml'
d['editorPassword']=next(line.split(':',1)[1].strip() for line in yaml.read_text().splitlines() if line.startswith('password:'))
p.write_text(json.dumps(d))
p.chmod(0o640)
os.chown(p,0,grp.getgrnam('browser').gr_gid)
PYCONTROL
sudo -u browser env WORKBENCH_LANGUAGE="$WORKBENCH_LANGUAGE" python3 - <<'PYCODE'
import json, os
from pathlib import Path
p = Path.home() / "profile/Default/Preferences"
p.parent.mkdir(parents=True, exist_ok=True)
d = json.loads(p.read_text()) if p.exists() else {}
d.setdefault("intl", {})["accept_languages"] = os.environ["WORKBENCH_LANGUAGE"]
d["intl"]["selected_languages"] = os.environ["WORKBENCH_LANGUAGE"]
d.setdefault("translate", {})["enabled"] = False
d["credentials_enable_service"] = False
d.setdefault("profile", {})["password_manager_enabled"] = False
p.write_text(json.dumps(d))
PYCODE
# Seed only appearance settings for a new editor profile. Existing preferences stay intact.
python3 - <<'PYEDITOR'
import shutil
from pathlib import Path
p = Path.home() / '.local/share/code-server/User/settings.json'
if not p.exists():
    p.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile('/etc/workbench/editor-theme.json', p)
PYEDITOR
# Systemd owns the editor independently of the SSH session and restarts crashes.
(umask 077; printf '%s' "$PWD" > "$HOME/.config/workbench/editor-workspace")
if ! systemctl is-active --quiet workbench-editor.service; then
  sudo systemctl start workbench-editor.service
fi
# The editor unit waits for HTTP readiness; browser ordering also covers direct
# systemctl starts and keeps the editor available while Chromium closes.
sudo systemctl reset-failed workbench-display workbench-window-manager workbench-browser workbench-webvnc 2>/dev/null || true
sudo systemctl reset-failed workbench-display-governor 2>/dev/null || true
sudo systemctl restart workbench-display workbench-window-manager workbench-browser workbench-webvnc
sudo python3 /usr/local/lib/workbench-wait-services.py
printf 'Workbench services ready. Timezone=%s Locale=%s Browser=%s\n' "$WORKBENCH_TZ" "$WORKBENCH_LOCALE" "$WORKBENCH_LANGUAGE"
