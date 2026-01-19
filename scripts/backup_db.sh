#!/bin/bash
set -e

OUTPUT_DIR="${1:-database_backups}"

mkdir -p "$OUTPUT_DIR"

TIMESTAMP=$(date "+%FT%T")

BACKUP_FILE="${OUTPUT_DIR}/journal_${TIMESTAMP}.db"

DB_FILE="${DB_PATH:-/data/journal.db}"

if [ ! -f "$DB_FILE" ]; then
    echo "Error: Database file '$DB_FILE' not found."
    exit 1
fi

echo "Backing up $DB_FILE to $BACKUP_FILE..."
sqlite3 "$DB_FILE" ".backup '$BACKUP_FILE'"

echo "Backup completed successfully."
