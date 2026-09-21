-- +goose Up
ALTER TABLE users ADD COLUMN translation TEXT NOT NULL DEFAULT 'ESV';

-- +goose Down
ALTER TABLE users DROP COLUMN translation;
