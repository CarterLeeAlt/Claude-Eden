# shellcheck shell=bash
# Shared by the base setup and tool updates. Callers validate Debian 13 first.
workbench_install_chromium() {
  workbench_cache_chromium --no-install-recommends chromium chromium-l10n "$@"
  sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${WORKBENCH_APT_CONFIG:-/dev/null}" apt-get install -y --no-install-recommends chromium chromium-l10n "$@" </dev/null
}
