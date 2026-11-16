#!/usr/bin/env bash
set -euo pipefail
# Existing environments can still open Desktop before their next maintenance
# update installs IBus. Their established committed-text path remains usable.
if ! command -v ibus-daemon >/dev/null || ! /usr/bin/python3 -c 'import gi; gi.require_version("IBus", "1.0")' 2>/dev/null; then
  exec "$@"
fi
umask 077
input_root="$HOME/.cache/workbench-input"
mkdir -p "$input_root"
chmod 700 "$input_root"
bus_socket="$input_root/bus"
export IBUS_ADDRESS="unix:path=$bus_socket"
export GTK_IM_MODULE=ibus QT_IM_MODULE=ibus XMODIFIERS=@im=ibus
bus_pid='' bridge_pid='' app_pid='' session_pid=''
cleanup() {
  trap '' TERM INT HUP
  if [ -n "$app_pid" ]; then
    kill -TERM "$app_pid" 2>/dev/null || true
    # Let Node close Chromium and flush its profile before closing the buses.
    wait "$app_pid" 2>/dev/null || true
  fi
  for pid in "$bridge_pid" "$bus_pid" "$session_pid"; do
    if [ -n "$pid" ]; then
      kill -TERM "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  rm -f "$input_root/ready" "$input_root/input.sock"
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
trap 'exit 129' HUP
# GTK's IBus portal needs a shared session bus. For the browser, allow only
# that service to activate, preserving its existing absence of a desktop
# keyring/portal session. Desktop keeps its already-owned keyring bus.
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
  mkdir -p "$input_root/services"
  cp /usr/share/dbus-1/services/org.freedesktop.portal.IBus.service "$input_root/services/"
  cat > "$input_root/session.conf" <<CONFIG
<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type><listen>unix:path=$input_root/session-bus</listen><auth>EXTERNAL</auth>
  <servicedir>$input_root/services</servicedir>
  <policy context="default"><allow send_destination="*"/><allow receive_sender="*"/><allow own="*"/></policy>
</busconfig>
CONFIG
  rm -f "$input_root/session-bus"
  dbus-daemon --nofork --config-file="$input_root/session.conf" >/dev/null 2>&1 &
  session_pid=$!
  while [ ! -S "$input_root/session-bus" ]; do
    kill -0 "$session_pid"
    sleep .05
  done
  export DBUS_SESSION_BUS_ADDRESS="unix:path=$input_root/session-bus"
fi
rm -f "$input_root/ready" "$bus_socket" "$input_root/input.sock"
ibus-daemon --single --panel=disable --config=disable --emoji-extension=disable --cache=none --address="$IBUS_ADDRESS" &
bus_pid=$!
while [ ! -S "$bus_socket" ]; do
  kill -0 "$bus_pid"
  if [ -n "$session_pid" ]; then kill -0 "$session_pid"; fi
  sleep .05
done
/usr/local/bin/workbench-native-input &
bridge_pid=$!
while [ ! -f "$input_root/ready" ]; do
  kill -0 "$bridge_pid"
  kill -0 "$bus_pid"
  if [ -n "$session_pid" ]; then kill -0 "$session_pid"; fi
  sleep .05
done
"$@" &
app_pid=$!
session_pids=("$app_pid" "$bridge_pid" "$bus_pid")
if [ -n "$session_pid" ]; then session_pids+=("$session_pid"); fi
status=0
exited=''
wait -n -p exited "${session_pids[@]}" || status=$?
if [ "$exited" != "$app_pid" ]; then
  echo 'An input session dependency stopped.' >&2
  if [ "$status" = 0 ]; then status=1; fi
fi
exit "$status"
