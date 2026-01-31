-- +goose Up
ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

-- +goose Down
ALTER TABLE users DROP COLUMN timezone;
