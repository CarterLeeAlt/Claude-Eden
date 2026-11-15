#!/bin/sh
# Only stock and Workbench-owned lists are managed. Custom lists and direct URIs
# remain under the user's control. The preference survives guest cloning.
sudo env APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" sh -se <<'WORKBENCH_DEBIAN_MIRRORS'
set -eu
preference=auto
if [ -f /etc/workbench/debian-mirror ]; then
  preference=$(cat /etc/workbench/debian-mirror)
fi
case "$preference" in
  auto|official) ;;
  *) echo 'Invalid /etc/workbench/debian-mirror: use auto or official.' >&2; exit 1;;
esac
for repository in debian debian-security; do
  list="/etc/apt/mirrors/$repository.list"
  [ -f "$list" ] || continue
  content=$(cat "$list")
  official="https://deb.debian.org/$repository"
  case "$content" in
    "$official"|'# Workbench managed Debian mirrors'*) ;;
    *) continue;;
  esac
  stage=$(mktemp "$list.XXXXXX")
  probe=$(mktemp)
  trap 'rm -f "$stage" "$probe"' EXIT
  printf '# Workbench managed Debian mirrors\n' > "$stage"
  suite=trixie
  [ "$repository" != debian-security ] || suite="$suite-security"
  priority=0
  # Prefer the official CDN when it works through the guest's configured
  # network. Probe domestic mirrors only after the official source fails.
  if [ "$preference" = auto ] && ! timeout 8 /usr/lib/apt/apt-helper -o Acquire::Retries=0 \
      -o Acquire::http::Timeout=5 -o Acquire::https::Timeout=5 \
      download-file "$official/dists/$suite/InRelease" "$probe" >/dev/null 2>&1; then
    for mirror in "https://mirrors.nju.edu.cn/$repository" "https://mirrors.ustc.edu.cn/$repository"; do
      rm -f "$probe"
      if timeout 8 /usr/lib/apt/apt-helper -o Acquire::Retries=0 \
          -o Acquire::http::Timeout=5 -o Acquire::https::Timeout=5 \
          download-file "$mirror/dists/$suite/InRelease" "$probe" >/dev/null 2>&1; then
        priority=$((priority + 1))
        printf '%s\tpriority:%s\n' "$mirror" "$priority" >> "$stage"
      else
        printf 'Debian mirror unavailable; using the next source: %s\n' "$mirror"
      fi
    done
  fi
  priority=$((priority + 1))
  printf '%s\tpriority:%s\n' "$official" "$priority" >> "$stage"
  chmod 0644 "$stage"
  mv -f "$stage" "$list"
  rm -f "$probe"
  trap - EXIT
done
WORKBENCH_DEBIAN_MIRRORS
