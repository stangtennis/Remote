#!/usr/bin/env bash
set -euo pipefail

credentials_file="${RD_CREDENTIALS_FILE:-$HOME/.config/remote-desktop/credentials.env}"
cli="${REMOTE_DESKTOP_CLI:-$HOME/.local/bin/remote-desktop-cli}"
state_dir="$HOME/.local/state/remote-desktop"
watch_log="$state_dir/support-watch.log"
real_opencode="${OPENCODE_REAL:-$HOME/.local/lib/node_modules/opencode-ai/bin/opencode.exe}"
ai_env="$HOME/projekter/aisupport/ai-controller.env"

if [[ -r "$ai_env" ]]; then
  set -a
  source "$ai_env"
  set +a
fi

if [[ ! -x "$real_opencode" ]]; then
  echo "OpenCode blev ikke fundet: $real_opencode" >&2
  exit 1
fi

if [[ -z "${RD_EMAIL:-}" || -z "${RD_PASSWORD:-}" ]] && [[ ! -f "$credentials_file" ]]; then
  if [[ ! -t 0 ]]; then
    echo "Første OpenCode-opsætning kræver en terminal til Supabase-login." >&2
    exit 1
  fi
  read -rp "Supabase email: " RD_EMAIL
  read -rsp "Supabase password: " RD_PASSWORD
  echo
  install -d -m 700 "$(dirname "$credentials_file")"
  umask 077
  printf 'RD_EMAIL=%s\nRD_PASSWORD=%s\n' "$RD_EMAIL" "$RD_PASSWORD" > "$credentials_file"
  chmod 600 "$credentials_file"
  unset RD_EMAIL RD_PASSWORD
  echo "Login gemt sikkert til $credentials_file"
fi

if [[ -x "$cli" ]]; then
  mkdir -p "$state_dir"
  if ! pgrep -u "$(id -u)" -f '[r]emote-desktop-cli support-watch' >/dev/null 2>&1; then
    nohup "$cli" support-watch >>"$watch_log" 2>&1 </dev/null &
  fi
fi

exec "$real_opencode" "$@"
