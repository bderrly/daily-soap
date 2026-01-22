FROM python:3.11-slim

RUN pip install --no-cache-dir google-cloud-storage

COPY scripts/backup_db.py /usr/local/bin/backup_db.py

RUN chmod +x /usr/local/bin/backup_db.py

ENTRYPOINT ["python", "/usr/local/bin/backup_db.py"]
