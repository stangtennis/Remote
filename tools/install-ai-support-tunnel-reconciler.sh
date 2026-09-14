#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $EUID -ne 0 ]]; then
  printf 'Run this installer as root.\n' >&2
  exit 1
fi

: "${SUPABASE_URL:?Set SUPABASE_URL before installing}"
: "${SUPABASE_SERVICE_ROLE_KEY:?Set SUPABASE_SERVICE_ROLE_KEY before installing}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

install -d -m 700 /etc/ai-support /usr/local/libexec
install -m 700 "$SCRIPT_DIR/ai-support-tunnel-reconciler.sh" /usr/local/libexec/ai-support-tunnel-reconciler.sh
install -m 644 "$SCRIPT_DIR/ai-support-tunnel-reconciler.service" /etc/systemd/system/ai-support-tunnel-reconciler.service
umask 077
printf 'SUPABASE_URL=%q\nSUPABASE_SERVICE_ROLE_KEY=%q\n' \
  "$SUPABASE_URL" "$SUPABASE_SERVICE_ROLE_KEY" > /etc/ai-support/tunnel-reconciler.env
chmod 600 /etc/ai-support/tunnel-reconciler.env
systemctl daemon-reload
systemctl enable ai-support-tunnel-reconciler.service
systemctl restart ai-support-tunnel-reconciler.service
systemctl --no-pager --full status ai-support-tunnel-reconciler.service
