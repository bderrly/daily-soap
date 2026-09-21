-- +goose Up
ALTER TABLE esv_cache RENAME TO scripture_cache;

-- +goose Down
ALTER TABLE scripture_cache RENAME TO esv_cache;
