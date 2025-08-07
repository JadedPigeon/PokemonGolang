-- +goose Up
ALTER TABLE pokedex
ADD COLUMN image_url TEXT;

-- +goose Down
ALTER TABLE pokedex
DROP COLUMN image_url;
