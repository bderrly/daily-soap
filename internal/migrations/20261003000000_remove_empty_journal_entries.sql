-- +goose Up
DELETE FROM journal
WHERE TRIM(observation) = ''
  AND TRIM(application) = ''
  AND TRIM(prayer) = ''
  AND (selected_verses IS NULL OR selected_verses = '[]' OR selected_verses = '' OR selected_verses = 'null');

-- +goose Down
