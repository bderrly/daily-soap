-- +goose Up
ALTER TABLE journal ADD COLUMN translation TEXT NOT NULL DEFAULT 'ESV';

-- +goose Down
ALTER TABLE journal DROP COLUMN translation;
