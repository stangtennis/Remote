#!/usr/bin/env bash
# Health check script for Remote Desktop infrastructure
# Checks: Supabase, TURN, Coturn, Caddy, Cloudflared

set -euo pipefail

FAIL=0

check() {
    local name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "[OK]   $name"
    else
        echo "[FAIL] $name"
        FAIL=1
    fi
}

# Supabase REST API
check "Supabase API" bash -c '
    code=$(curl -s -o /dev/null -w "%{http_code}" \
        https://supabase.hawkeye123.dk/rest/v1/ \
        -H "apikey: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE" \
        --connect-timeout 5 --max-time 10)
    [ "$code" -ge 200 ] && [ "$code" -lt 400 ]
'

# TURN server
check "TURN server (turn.hawkeye123.dk:3478)" nc -z -w 3 turn.hawkeye123.dk 3478

# Coturn Docker container
check "Coturn container" bash -c '
    status=$(docker ps --filter name=coturn --format "{{.Status}}")
    [ -n "$status" ]
'

# Caddy / updates endpoint
check "Caddy (updates.hawkeye123.dk)" bash -c '
    code=$(curl -s -o /dev/null -w "%{http_code}" \
        https://updates.hawkeye123.dk/version.json \
        --connect-timeout 5 --max-time 10)
    [ "$code" -ge 200 ] && [ "$code" -lt 400 ]
'

# Cloudflared tunnel
check "Cloudflared tunnel" systemctl is-active cloudflared

exit $FAIL
