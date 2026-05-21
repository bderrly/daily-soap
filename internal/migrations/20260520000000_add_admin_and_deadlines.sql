-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys=off;
BEGIN TRANSACTION;

CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_verified INTEGER DEFAULT 0,
    verification_token TEXT,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    is_admin INTEGER DEFAULT 0 NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    verified_at DATETIME
);

INSERT INTO users_new (id, email, password_hash, is_verified, verification_token, timezone, is_admin, created_at, verified_at)
SELECT id, email, password_hash, is_verified, verification_token, timezone, 0, CURRENT_TIMESTAMP, NULL
FROM users;

UPDATE users_new SET verified_at = created_at WHERE is_verified = 1;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

COMMIT;
PRAGMA foreign_keys=on;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
PRAGMA foreign_keys=off;
BEGIN TRANSACTION;

CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_verified INTEGER DEFAULT 0,
    verification_token TEXT,
    timezone TEXT NOT NULL DEFAULT 'UTC'
);

INSERT INTO users_new (id, email, password_hash, is_verified, verification_token, timezone)
SELECT id, email, password_hash, is_verified, verification_token, timezone
FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

COMMIT;
PRAGMA foreign_keys=on;
-- +goose StatementEnd
