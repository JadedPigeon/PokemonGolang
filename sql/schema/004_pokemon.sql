-- +goose Up
ALTER TABLE pokemon_moves
ADD COLUMN type TEXT NOT NULL DEFAULT 'normal',
ADD COLUMN description TEXT;

-- +goose Down
ALTER TABLE pokemon_moves
DROP COLUMN description,
DROP COLUMN type;
