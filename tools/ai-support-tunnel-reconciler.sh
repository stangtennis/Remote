#!/usr/bin/env bash
set -Eeuo pipefail

CONFIG_FILE="${AI_SUPPORT_RECONCILER_CONFIG:-/etc/ai-support/tunnel-reconciler.env}"
KEYS_FILE="${AI_SUPPORT_AUTHORIZED_KEYS:-/home/dennis/.ssh/authorized_keys}"
POLL_SECONDS="${AI_SUPPORT_RECONCILE_INTERVAL:-15}"
OPERATOR_KEY="${AI_SUPPORT_OPERATOR_KEY:-/home/dennis/.ssh/id_rsa}"

if [[ ! -r "$CONFIG_FILE" ]]; then
  printf 'Missing reconciler config: %s\n' "$CONFIG_FILE" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$CONFIG_FILE"

: "${SUPABASE_URL:?SUPABASE_URL is required}"
: "${SUPABASE_SERVICE_ROLE_KEY:?SUPABASE_SERVICE_ROLE_KEY is required}"

if [[ $EUID -ne 0 ]]; then
  printf 'The AI-support tunnel reconciler must run as root.\n' >&2
  exit 1
fi

api_url="${SUPABASE_URL%/}/rest/v1/ai_support_clients?select=client_id,status,tunnel_port,windows_ssh_user,windows_ssh_port"
rpc_url="${SUPABASE_URL%/}/rest/v1/rpc/complete_ai_support_uninstall"
tmp_json=""
trap '[[ -n "$tmp_json" ]] && rm -f "$tmp_json"' EXIT

terminate_client_sessions() {
  local client_id="$1"
  local environ pid env_data
  for environ in /proc/[0-9]*/environ; do
    [[ -r "$environ" ]] || continue
    pid="${environ#/proc/}"
    pid="${pid%/environ}"
    env_data="$(cat "$environ" 2>/dev/null | tr '\0' '\n')" || continue
    if grep -Fqx "AI_SUPPORT_CLIENT_ID=$client_id" <<< "$env_data"; then
      kill -TERM "$pid" 2>/dev/null || true
      sleep 1
      kill -KILL "$pid" 2>/dev/null || true
      logger -t ai-support-tunnel-reconciler "terminated revoked tunnel client=$client_id pid=$pid"
    fi
  done
}

remove_client_key() {
  local client_id="$1"
  local marker="${client_id}-tunnel"
  local temporary
  [[ -f "$KEYS_FILE" ]] || return 0
  grep -Fq -- "$marker" "$KEYS_FILE" || return 0
  temporary="$(mktemp "${KEYS_FILE}.reconcile.XXXXXX")"
  if ! awk -v marker="$marker" '$NF != marker { print }' "$KEYS_FILE" > "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  chown --reference="$KEYS_FILE" "$temporary"
  chmod --reference="$KEYS_FILE" "$temporary"
  mv -f "$temporary" "$KEYS_FILE"
  logger -t ai-support-tunnel-reconciler "removed revoked tunnel key client=$client_id"
}

uninstall_client() {
  local client_id="$1" tunnel_port="$2" windows_user="$3"
  [[ "$tunnel_port" =~ ^[0-9]+$ ]] || return 0
  [[ "$windows_user" =~ ^[A-Za-z0-9._-]{1,32}$ ]] || return 1
  [[ -r "$OPERATOR_KEY" ]] || return 1
  local cleanup_command
  cleanup_command="\$task='AI-Support-Persistent-Tunnel'; Stop-ScheduledTask -TaskName \$task -ErrorAction SilentlyContinue; Unregister-ScheduledTask -TaskName \$task -Confirm:\$false -ErrorAction SilentlyContinue; Stop-Service -Name sshd -Force -ErrorAction SilentlyContinue; \$backup='C:\\ProgramData\\AI-Support\\sshd_config.backup'; \$config='C:\\ProgramData\\ssh\\sshd_config'; if (Test-Path \$backup) { Copy-Item -LiteralPath \$backup -Destination \$config -Force }; \$policy='HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System'; \$previous='C:\\ProgramData\\AI-Support\\token-policy.backup'; if (Test-Path \$previous) { \$value=(Get-Content -LiteralPath \$previous -Raw).Trim(); if (\$value -eq 'MISSING') { Remove-ItemProperty -Path \$policy -Name LocalAccountTokenFilterPolicy -ErrorAction SilentlyContinue } else { Set-ItemProperty -Path \$policy -Name LocalAccountTokenFilterPolicy -Value ([int]\$value) -Type DWord } }; Remove-LocalUser -Name '$windows_user' -ErrorAction SilentlyContinue; Remove-Item -LiteralPath 'C:\\ProgramData\\AI-Support' -Recurse -Force -ErrorAction SilentlyContinue"
  if ! ssh -n -T -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null -o IdentitiesOnly=yes -i "$OPERATOR_KEY" \
    -p "$tunnel_port" "$windows_user@127.0.0.1" "$cleanup_command" >/dev/null 2>&1; then
    logger -t ai-support-tunnel-reconciler "client cleanup failed client=$client_id"
    return 1
  fi
  logger -t ai-support-tunnel-reconciler "client cleanup completed client=$client_id"
}

delete_client_record() {
  local client_id="$1"
  curl --fail --silent --show-error --connect-timeout 10 --max-time 20 \
    -H "apikey: $SUPABASE_SERVICE_ROLE_KEY" \
    -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
    -H 'Content-Type: application/json' \
    -d "{\"p_client_id\":\"$client_id\"}" "$rpc_url" >/dev/null
}

reconcile_once() {
  local revoked_clients client_id status tunnel_port windows_user windows_port
  tmp_json="$(mktemp /run/ai-support-tunnel-reconciler.XXXXXX.json)"
  if ! curl --fail --silent --show-error --connect-timeout 10 --max-time 20 \
    -H "apikey: $SUPABASE_SERVICE_ROLE_KEY" \
    -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
    "$api_url" > "$tmp_json"; then
    logger -t ai-support-tunnel-reconciler 'Supabase lookup failed; leaving keys unchanged'
    rm -f "$tmp_json"
    tmp_json=""
    return 1
  fi
  if ! jq -e 'type == "array" and all(.[]; (.client_id | type == "string") and (.status | type == "string"))' "$tmp_json" >/dev/null; then
    logger -t ai-support-tunnel-reconciler 'Supabase returned invalid client data; leaving keys unchanged'
    rm -f "$tmp_json"
    tmp_json=""
    return 1
  fi

  revoked_clients="$(jq -r '.[] | select(.status == "revoked" or .status == "uninstall_pending") | [.client_id, (.status // ""), (.tunnel_port // ""), (.windows_ssh_user // ""), (.windows_ssh_port // "")] | @tsv' "$tmp_json")"
  while IFS=$'\t' read -r client_id status tunnel_port windows_user windows_port; do
    [[ -n "$client_id" ]] || continue
    [[ "$client_id" =~ ^ai-[a-z0-9]{8,32}$ ]] || continue
    if [[ "$status" == "uninstall_pending" && -n "$tunnel_port" ]]; then
      uninstall_client "$client_id" "$tunnel_port" "$windows_user" || continue
    fi
    remove_client_key "$client_id"
    terminate_client_sessions "$client_id"
    delete_client_record "$client_id" || logger -t ai-support-tunnel-reconciler "client delete failed client=$client_id"
  done <<< "$revoked_clients"

  rm -f "$tmp_json"
  tmp_json=""
  return 0
}

while :; do
  reconcile_once || true
  sleep "$POLL_SECONDS"
done
