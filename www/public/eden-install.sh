#!/bin/sh
# Claude Eden download/installation plan. No application or profile is changed.
# sh eden-install.sh plan [--version TAG] [--platform win32-amd64]
# sh eden-install.sh download --destination /absolute/path [--version TAG]
set -eu
if ! command -v node >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
  printf '%s\n' '{"protocol":1,"status":"error","error":{"code":"dependency_missing","message":"Claude Eden bootstrap requires Node.js 20+ and curl. Read https://github.com/CarterLeeAlt/Claude-Eden/blob/main/AGENT-USE.md","next_action":"install_dependencies"}}'
  exit 1
fi
eden_bootstrap_tmp=$(mktemp -d)
trap 'rm -rf "$eden_bootstrap_tmp"' EXIT
trap 'exit 1' HUP INT TERM
if ! curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL --connect-timeout 15 --max-time 120 \
  https://raw.githubusercontent.com/CarterLeeAlt/Claude-Eden/main/www/public/agent-bootstrap.mjs -o "$eden_bootstrap_tmp/bootstrap.mjs"; then
  printf '%s\n' '{"protocol":1,"status":"error","error":{"code":"download_failed","message":"Could not download the public Claude Eden bootstrap. No application or profile was changed.","next_action":"check_connectivity"}}'
  exit 1
fi
node "$eden_bootstrap_tmp/bootstrap.mjs" "$@"
