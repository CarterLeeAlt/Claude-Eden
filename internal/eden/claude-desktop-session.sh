#!/usr/bin/env bash
set -euo pipefail
# The session owns its dependencies. systemd signals only the log runner,
# which forwards the signal here; the application exits before its services.
WORKBENCH_NOTIFY_SOCKET="$NOTIFY_SOCKET"
unset NOTIFY_SOCKET
children=()
child_names=()
cleanup() {
  status=$?
  trap - EXIT
  trap '' TERM INT HUP
  for ((index=${#children[@]}-1; index>=0; index--)); do
    pid=${children[index]}
    if kill -0 "$pid" 2>/dev/null; then kill -TERM "$pid" 2>/dev/null || true; fi
    child_status=0
    wait "$pid" || child_status=$?
    case "$child_status" in
      0|129|130|143) ;;
      *) echo "${child_names[index]} exited with status $child_status" >&2
         if [ "$status" = 0 ]; then status=$child_status; fi ;;
    esac
  done
  exit "$status"
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
trap 'exit 129' HUP
check_children() {
  for ((index=0; index<${#children[@]}; index++)); do
    if ! kill -0 "${children[index]}" 2>/dev/null; then
      child_status=0
      wait "${children[index]}" || child_status=$?
      echo "${child_names[index]} stopped before the session was ready (status $child_status)." >&2
      return 1
    fi
  done
}
for command in dbus-daemon dbus-update-activation-environment xauth Xtigervnc xdpyinfo openbox gnome-keyring-daemon gdbus tigervncpasswd xprop systemd-notify timeout; do
  command -v "$command" >/dev/null
done
export DISPLAY=:78 XAUTHORITY=/run/workbench-desktop/Xauthority
export XDG_RUNTIME_DIR=/run/workbench-desktop XDG_CURRENT_DESKTOP=Openbox
export PATH="$HOME/.local/bin:$PATH" BROWSER=/usr/local/bin/workbench-open
umask 077
dbus-daemon --session --nofork --print-address=1 > "$XDG_RUNTIME_DIR/bus-address" &
children+=("$!"); child_names+=(D-Bus)
while [ ! -s "$XDG_RUNTIME_DIR/bus-address" ]; do check_children; sleep .1; done
DBUS_SESSION_BUS_ADDRESS=$(cat "$XDG_RUNTIME_DIR/bus-address")
export DBUS_SESSION_BUS_ADDRESS
dbus-update-activation-environment DISPLAY XAUTHORITY XDG_RUNTIME_DIR XDG_CURRENT_DESKTOP
touch "$XAUTHORITY"
xauth -f "$XAUTHORITY" add "$DISPLAY" . "$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
password="$HOME/.config/workbench/desktop-password"
if [ ! -s "$password" ]; then
  mkdir -p "$(dirname "$password")"
  od -An -N4 -tx1 /dev/urandom | tr -d ' \n' > "$password"
fi
tigervncpasswd -f < "$password" > "$XDG_RUNTIME_DIR/vnc-password"
Xtigervnc "$DISPLAY" -geometry 1600x1000 -depth 24 -rfbport -1 -rfbunixpath "$XDG_RUNTIME_DIR/vnc.sock" -rfbunixmode 0600 -nolisten tcp -auth "$XAUTHORITY" -SecurityTypes VncAuth -PasswordFile "$XDG_RUNTIME_DIR/vnc-password" -AlwaysShared &
display_pid=$!
children+=("$display_pid"); child_names+=(display)
while ! timeout 2 xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; do
  check_children
  sleep .1
done
openbox --config-file /etc/workbench/desktop-openbox.xml &
children+=("$!"); child_names+=(Openbox)
while ! timeout 2 xprop -root _NET_SUPPORTING_WM_CHECK 2>/dev/null | grep -q '0x[1-9a-f]'; do
  check_children
  sleep .1
done
# Lower the refresh rate of unattended animation. Optional on older guests.
if [ -x /usr/local/bin/workbench-display-governor ]; then
  /usr/local/bin/workbench-display-governor &
  children+=("$!"); child_names+=(display-governor)
fi
mkfifo -m 0600 "$XDG_RUNTIME_DIR/unlock"
gnome-keyring-daemon --foreground --unlock --components=secrets < "$XDG_RUNTIME_DIR/unlock" >/dev/null &
children+=("$!"); child_names+=(keyring)
# Do not replace or erase an existing keyring when the supplied key is wrong.
while true; do
  check_children
  # Query the bus first so a slow unlock never auto-starts a second daemon.
  if timeout 2 gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus --method org.freedesktop.DBus.NameHasOwner org.freedesktop.secrets 2>/dev/null | grep -q true &&
     login_state=$(timeout 2 gdbus call --session --dest org.freedesktop.secrets --object-path /org/freedesktop/secrets/aliases/login --method org.freedesktop.DBus.Properties.Get org.freedesktop.Secret.Collection Locked 2>/dev/null) &&
     default_state=$(timeout 2 gdbus call --session --dest org.freedesktop.secrets --object-path /org/freedesktop/secrets/aliases/default --method org.freedesktop.DBus.Properties.Get org.freedesktop.Secret.Collection Locked 2>/dev/null); then
    if [[ "$login_state" != *false* || "$default_state" != *false* ]]; then
      echo '无法解锁已有密钥环，登录数据未修改。' >&2; exit 1
    fi
    break
  fi
  sleep .1
done
rm "$XDG_RUNTIME_DIR/unlock"
touch "$XDG_RUNTIME_DIR/keyring-ready"
# Record the running application's density for clients that reconnect later.
display_scale=2
if [[ -r /etc/workbench/display-scale && $(< /etc/workbench/display-scale) == 1.5 ]]; then
  display_scale=1.5
fi
printf '%s\n' "$display_scale" > "$XDG_RUNTIME_DIR/display-scale"
printf '%s\n' "$display_scale" > "$XDG_RUNTIME_DIR/base-display-scale"
scale_args=(--force-device-scale-factor="$display_scale")
if command -v xsettingsd >/dev/null; then
  dpi=$(awk -v scale="$display_scale" 'BEGIN {printf "%.0f", scale * 96 * 1024}')
  # Settings that stay as they are when a viewer changes the density.
  # Workbench owns the embedded app's window controls; keep its native buttons hidden.
  printf 'Gtk/DecorationLayout ":"\n' > "$XDG_RUNTIME_DIR/xsettings.fixed"
  if [ "$display_scale" = 1.5 ]; then
    # Favoring smoothness gives up the app's animations too: while one plays,
    # the app repaints its whole message area on every frame.
    printf 'Gtk/EnableAnimations 0\n' >> "$XDG_RUNTIME_DIR/xsettings.fixed"
  fi
  { printf 'Xft/DPI %s\nGdk/WindowScalingFactor 1\nGdk/UnscaledDPI %s\n' "$dpi" "$dpi"; cat "$XDG_RUNTIME_DIR/xsettings.fixed"; } > "$XDG_RUNTIME_DIR/xsettings.conf"
  xsettingsd --config="$XDG_RUNTIME_DIR/xsettings.conf" &
  printf '%s\n' "$!" > "$XDG_RUNTIME_DIR/xsettings.pid"
  children+=("$!"); child_names+=(display-settings)
  while true; do
    probe_status=0
    timeout 2 /usr/bin/python3 - <<'WORKBENCH_DESKTOP_DPI_READY' || probe_status=$?
import sys
from Xlib import display
connection = display.Display()
selection = connection.intern_atom('_XSETTINGS_S0')
ready = bool(connection.get_selection_owner(selection))
connection.close()
sys.exit(0 if ready else 75)
WORKBENCH_DESKTOP_DPI_READY
    case "$probe_status" in
      0) break ;;
      75|124) ;;
      *) echo "Desktop display settings probe failed (status $probe_status)." >&2; exit "$probe_status" ;;
    esac
    check_children
    sleep .1
  done
  scale_args=()
fi
/usr/local/bin/workbench-desktop-files &
files_pid=$!
children+=("$files_pid"); child_names+=(desktop-files)
while [ ! -S "$XDG_RUNTIME_DIR/files.sock" ]; do
  check_children
  sleep .1
done
export GTK_USE_PORTAL=1
# Openbox is not auto-detected as a libsecret desktop by Electron. Select the
# unlocked keyring explicitly so OAuth tokens survive application restarts.
/usr/local/bin/workbench-input-session /usr/bin/python3 -I /usr/local/bin/client_runtime.py claude-desktop "${scale_args[@]}" --password-store=gnome-libsecret --xdg-portal-required-version=3 &
app_pid=$!
children+=("$app_pid"); child_names+=(Claude-Desktop)
# The service becomes ready only after the application has opened a window.
while true; do
  check_children
  if timeout 2 xprop -root _NET_CLIENT_LIST 2>/dev/null | grep -q '0x[1-9a-f]'; then
    NOTIFY_SOCKET="$WORKBENCH_NOTIFY_SOCKET" systemd-notify --ready
    break
  fi
  sleep .1
done
status=0
exited=''
wait -n -p exited "${children[@]}" || status=$?
if [ "$exited" != "$app_pid" ]; then
  echo 'A Claude Desktop session dependency stopped.' >&2
  if [ "$status" = 0 ]; then status=1; fi
fi
exit "$status"
