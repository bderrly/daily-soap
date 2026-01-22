import datetime
import logging
import os
import posixpath
import sqlite3
import sys
import tempfile
from contextlib import closing
from pathlib import Path

from google.cloud import storage

logging.basicConfig(level=logging.INFO, format='%(levelname)s: %(message)s')
logger = logging.getLogger(__name__)

def backup_database():
    db_path = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(os.environ.get("DB_PATH", "app.db"))
    bucket_name = os.environ.get("GCS_BUCKET")
    backup_dir = os.environ.get("BACKUP_DIR", "backups")

    if not bucket_name:
        logger.error("GCS_BUCKET environment variable is required.")
        sys.exit(1)

    if not db_path.exists():
        logger.error(f"Database file '{db_path}' not found.")
        sys.exit(1)

    now = datetime.datetime.now(datetime.timezone.utc)
    blob_name = now.strftime("app_%Y-%m-%dT%H-%M-%SZ.db")
    full_blob_path = posixpath.join(backup_dir, blob_name)

    logger.info(f"Starting backup of {db_path}...")

    try:
        with tempfile.NamedTemporaryFile(suffix=".db") as temp_backup:
            # Use closing() to ensure DB handles are released on error.
            with closing(sqlite3.connect(db_path)) as src, \
                 closing(sqlite3.connect(temp_backup.name)) as dst:
                src.backup(dst)
            
            logger.info(f"Local backup created at {temp_backup.name}")

            logger.info(f"Uploading to gs://{bucket_name}/{full_blob_path}...")
            storage_client = storage.Client()
            bucket = storage_client.bucket(bucket_name)
            blob = bucket.blob(full_blob_path)
            blob.upload_from_filename(temp_backup.name)
            logger.info("Upload complete.")

    except Exception:
        logger.exception("An unexpected error occurred during backup")
        sys.exit(1)

if __name__ == "__main__":
    backup_database()