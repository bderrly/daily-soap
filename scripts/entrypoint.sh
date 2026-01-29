#!/bin/sh
set -e

# Restore the database if a replica exists
# We use -if-replica-exists so it doesn't fail on the very first run when no backup exists yet.
echo "Attempting to restore database from object storage..."
litestream restore -if-replica-exists -config /etc/litestream.yml /data/app.db

# Run litestream replicate, which in turn starts the application via -exec
# When the application exits (e.g. SIGTERM), litestream will sync one last time and exit.
echo "Starting application with Litestream replication..."
exec litestream replicate -config /etc/litestream.yml -exec "./soap-journal"
