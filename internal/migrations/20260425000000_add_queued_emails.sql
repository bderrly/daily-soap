-- +goose Up
CREATE TABLE queued_emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    recipient TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, sent, failed
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at DATETIME,
    next_attempt_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_queued_emails_status_next_attempt ON queued_emails(status, next_attempt_at);

-- +goose Down
DROP TABLE queued_emails;
