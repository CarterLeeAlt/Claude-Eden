#!/usr/bin/env bash
# Build a guest-only payload with an independent Tailscale dependency graph.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
[[ $# == 3 && $3 == /* ]] || { echo 'Usage: build-tailnet.sh OS ARCH /absolute/output' >&2; exit 2; }
os=$1 arch=$2 destination=$3
case "$os/$arch" in linux/amd64|linux/arm64) ;; *) exit 2 ;; esac
mkdir -p "$(dirname "$destination")"
cd "$root/tools/tailnet"
GOOS=$os GOARCH=$arch CGO_ENABLED=0 GOFLAGS= "${GO:-go}" build -trimpath -buildvcs=false -ldflags '-s -w' -o "$destination" .
