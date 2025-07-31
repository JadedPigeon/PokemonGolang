-- +goose Up
ALTER TABLE pokemon_moves
ADD COLUMN move_id INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pokemon_moves
DROP COLUMN move_id;
