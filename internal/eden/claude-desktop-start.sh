#!/usr/bin/env bash
set -euo pipefail
trap 'exit 143' TERM
trap 'exit 130' INT
trap 'exit 129' HUP
check_session() {
  state=$(systemctl show workbench-claude-desktop.service --property=ActiveState --value)
  case "$state" in
    active|activating) ;;
    *) echo "Claude Desktop 会话已停止（$state）；请查看应用日志。" >&2; return 1 ;;
  esac
}
if systemctl is-active --quiet workbench-claude-desktop.service; then
  test -f /run/workbench-desktop/keyring-ready || { echo '请先保存并关闭现有 Claude Desktop，再使用工作台账户重新打开。' >&2; exit 1; }
  exit 0
fi
if systemctl is-failed --quiet workbench-claude-desktop.service; then
  sudo systemctl reset-failed workbench-claude-desktop.service
fi
sudo systemctl start --no-block workbench-claude-desktop.service
while [ ! -p /run/workbench-desktop/unlock ]; do
  check_session
  sleep .1
done
# The internal random key comes exclusively from SSH stdin, never argv or disk.
cat <&0 > /run/workbench-desktop/unlock &
writer=$!
cleanup_writer() {
  if kill -0 "$writer" 2>/dev/null; then kill "$writer" 2>/dev/null || true; fi
  wait "$writer" 2>/dev/null || true
}
trap cleanup_writer EXIT
while kill -0 "$writer" 2>/dev/null; do
  check_session
  sleep .1
done
wait "$writer"
trap - EXIT
while true; do
  if systemctl is-active --quiet workbench-claude-desktop.service; then exit 0; fi
  check_session
  sleep .1
done
