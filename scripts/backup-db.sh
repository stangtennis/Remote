#!/usr/bin/env bash
# Supabase PostgreSQL backup script
# Dumps the local Supabase Docker DB, gzips it, and prunes backups older than 7 days

set -euo pipefail

BACKUP_DIR="/home/dennis/backups/supabase"
RETENTION_DAYS=7
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/supabase_${TIMESTAMP}.sql.gz"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

# Ensure backup directory exists
if [ ! -d "$BACKUP_DIR" ]; then
    log "Creating backup directory: $BACKUP_DIR"
    mkdir -p "$BACKUP_DIR"
fi

# Run pg_dump and compress
log "Starting Supabase database backup..."
if docker exec supabase-db pg_dump -U supabase -d postgres --clean --if-exists | gzip > "$BACKUP_FILE"; then
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    log "Backup complete: $BACKUP_FILE ($SIZE)"
else
    log "ERROR: Backup failed!"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Prune old backups
log "Pruning backups older than ${RETENTION_DAYS} days..."
DELETED=$(find "$BACKUP_DIR" -name 'supabase_*.sql.gz' -type f -mtime +${RETENTION_DAYS} -print -delete | wc -l)
log "Deleted $DELETED old backup(s)"

# Summary
TOTAL=$(find "$BACKUP_DIR" -name 'supabase_*.sql.gz' -type f | wc -l)
log "Total backups remaining: $TOTAL"
