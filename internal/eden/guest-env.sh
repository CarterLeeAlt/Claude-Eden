#!/usr/bin/env bash
set -euo pipefail
sudo ln -sf "/usr/share/zoneinfo/$WORKBENCH_TZ" /etc/localtime
printf '%s\n' "$WORKBENCH_TZ" | sudo tee /etc/timezone >/dev/null
if ! locale -a | tr '[:upper:]' '[:lower:]' | grep -qx "${WORKBENCH_LOCALE,,}" && ! locale -a | grep -q "${WORKBENCH_LOCALE%%.*}.utf8"; then
  printf '%s UTF-8\n' "$WORKBENCH_LOCALE" | sudo tee -a /etc/locale.gen >/dev/null
  sudo locale-gen >/dev/null
fi
printf 'TZ=%s\nLANG=%s\nLC_ALL=%s\nWORKBENCH_LANGUAGE=%s\nBROWSER=/usr/local/bin/workbench-open\nPULSE_SERVER=unix:/run/workbench-audio/native\n' "$WORKBENCH_TZ" "$WORKBENCH_LOCALE" "$WORKBENCH_LOCALE" "$WORKBENCH_LANGUAGE" | sudo tee /etc/workbench/environment >/dev/null
sudo update-locale "LANG=$WORKBENCH_LOCALE" "LC_ALL=$WORKBENCH_LOCALE"
set -a
. /etc/workbench/environment
set +a
unset HTTP_PROXY HTTPS_PROXY ALL_PROXY FTP_PROXY NO_PROXY http_proxy https_proxy all_proxy ftp_proxy no_proxy

# Only the scoped installer receives this variable; services load their fixed environment.
if [ -n "${WORKBENCH_INSTALL_PROXY:-}" ]; then
  export http_proxy="$WORKBENCH_INSTALL_PROXY" https_proxy="$WORKBENCH_INSTALL_PROXY" HTTP_PROXY="$WORKBENCH_INSTALL_PROXY" HTTPS_PROXY="$WORKBENCH_INSTALL_PROXY"
fi
