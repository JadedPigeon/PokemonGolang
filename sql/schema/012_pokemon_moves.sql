-- +goose Up
ALTER TABLE pokemon_moves
  ALTER COLUMN pokemon_id SET NOT NULL,
  ALTER COLUMN move_id   SET NOT NULL;

-- +goose Down
ALTER TABLE pokemon_moves
  ALTER COLUMN pokemon_id DROP NOT NULL,
  ALTER COLUMN move_id   DROP NOT NULL;
