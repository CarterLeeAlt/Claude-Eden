# A virtual output shared by Workbench applications. No hardware or microphone
# modules are loaded, and the PulseAudio protocol is Unix-socket only.
if ! command -v pulseaudio >/dev/null || ! command -v parec >/dev/null; then
  sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get -o DPkg::Lock::Timeout=60 update </dev/null
  sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get -o DPkg::Lock::Timeout=60 install -y --no-install-recommends pulseaudio pulseaudio-utils </dev/null
fi
getent group workbench-audio >/dev/null || sudo groupadd --system workbench-audio
sudo usermod -a -G workbench-audio developer
sudo usermod -a -G workbench-audio browser
sudo install -d -m 0755 /etc/workbench /etc/pulse/client.conf.d
sudo tee /etc/pulse/client.conf.d/99-workbench.conf >/dev/null <<'CLIENT'
default-server = unix:/run/workbench-audio/native
autospawn = no
CLIENT
sudo tee /etc/workbench/audio.pa >/dev/null <<'PULSE'
load-module module-null-sink sink_name=workbench_output rate=48000 channels=2 format=s16le sink_properties=device.description=Workbench
load-module module-native-protocol-unix socket=/run/workbench-audio/native auth-anonymous=0 auth-cookie-enabled=0 auth-group-enable=1 auth-group=workbench-audio
set-default-sink workbench_output
set-default-source workbench_output.monitor
PULSE
sudo tee /etc/systemd/system/workbench-audio.service >/dev/null <<'UNIT'
[Unit]
Description=Workbench virtual audio output
[Service]
User=developer
Group=workbench-audio
Environment=HOME=/home/developer
Environment=PULSE_RUNTIME_PATH=/run/workbench-audio/runtime
Environment=PULSE_STATE_PATH=/run/workbench-audio/state
RuntimeDirectory=workbench-audio
RuntimeDirectoryMode=0750
ExecStart=/usr/local/bin/workbench-log-run /usr/bin/pulseaudio -n --daemonize=no --exit-idle-time=-1 --disallow-exit --disallow-module-loading --disable-shm=true --realtime=no --high-priority=no --use-pid-file=no --file=/etc/workbench/audio.pa
LogNamespace=workbench
Restart=on-failure
RestartSec=1
TimeoutStopSec=5
NoNewPrivileges=true
UNIT
sudo chmod 0644 /etc/workbench/audio.pa /etc/pulse/client.conf.d/99-workbench.conf /etc/systemd/system/workbench-audio.service
sudo systemctl daemon-reload
sudo systemctl reset-failed workbench-audio 2>/dev/null || true
sudo systemctl restart workbench-audio
for i in $(seq 1 50); do
  if pactl --server=unix:/run/workbench-audio/native info >/dev/null 2>&1; then break; fi
  sleep .1
done
pactl --server=unix:/run/workbench-audio/native info >/dev/null
