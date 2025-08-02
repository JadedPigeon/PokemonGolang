-- +goose Up
-- First drop the existing FK if it exists (adjust name if needed)
ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_challenge_pokemon_id_fkey;

-- Re-add with ON DELETE SET NULL and deferred checking
ALTER TABLE users
ADD CONSTRAINT users_challenge_pokemon_id_fkey
FOREIGN KEY (challenge_pokemon_id)
REFERENCES challenger_pokemon(id)
ON DELETE SET NULL
DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
-- Drop the deferred FK
ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_challenge_pokemon_id_fkey;

-- Re-add with original behavior (adjust as needed)
ALTER TABLE users
ADD CONSTRAINT users_challenge_pokemon_id_fkey
FOREIGN KEY (challenge_pokemon_id)
REFERENCES challenger_pokemon(id);
