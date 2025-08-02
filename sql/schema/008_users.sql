-- +goose Up
ALTER TABLE users
ADD COLUMN challenge_pokemon_id UUID REFERENCES challenger_pokemon(id);

-- +goose Down
ALTER TABLE users
DROP COLUMN challenge_pokemon_id;
