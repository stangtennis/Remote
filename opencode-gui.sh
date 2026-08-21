#!/usr/bin/env bash
set -euo pipefail

cli="${REMOTE_DESKTOP_CLI:-$HOME/.local/bin/remote-desktop-cli}"
opencode_bin="${OPENCODE_BIN:-$(command -v opencode || true)}"
state_dir="$HOME/.local/state/remote-desktop"
watch_log="$state_dir/support-watch.log"

if [[ -z "$opencode_bin" ]]; then
  echo "OpenCode blev ikke fundet. Sæt OPENCODE_BIN eller installer opencode." >&2
  exit 1
fi

if [[ -x "$cli" ]]; then
  mkdir -p "$state_dir"
  if ! pgrep -u "$(id -u)" -f '[r]emote-desktop-cli support-watch' >/dev/null 2>&1; then
    nohup "$cli" support-watch >>"$watch_log" 2>&1 </dev/null &
    echo "Remote support watcher startet. Log: $watch_log"
  fi
else
  echo "Remote support watcher ikke startet: RD_EMAIL/RD_PASSWORD mangler eller CLI mangler." >&2
fi

exec "$opencode_bin" "$@"
