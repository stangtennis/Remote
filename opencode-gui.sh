#!/usr/bin/env bash
set -euo pipefail

cli="${REMOTE_DESKTOP_CLI:-$HOME/.local/bin/remote-desktop-cli}"
opencode_bin="${OPENCODE_BIN:-$HOME/.local/bin/opencode}"
state_dir="$HOME/.local/state/remote-desktop"
watch_log="$state_dir/support-watch.log"
ai_env="$HOME/projekter/aisupport/ai-controller.env"

if [[ -r "$ai_env" ]]; then
  set -a
  # The key is kept outside the repository and inherited only by this AI shell.
  source "$ai_env"
  set +a
fi

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
  echo "Remote support CLI blev ikke fundet: $cli" >&2
fi

exec "$opencode_bin" "$@"
