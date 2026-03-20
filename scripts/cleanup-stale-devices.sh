#!/bin/bash
# Cleanup stale devices not seen in 90 days
# This is a fallback if pg_cron is not available
set -euo pipefail

RESULT=$(docker exec supabase-db psql -U supabase_admin -d postgres -t -c \
  "DELETE FROM remote_devices WHERE last_seen < NOW() - INTERVAL '90 days' AND is_online = false RETURNING device_id;")

if [ -n "$RESULT" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Cleaned up stale devices:$RESULT"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] No stale devices to clean up"
fi
