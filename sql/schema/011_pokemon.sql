-- +goose Up
-- Ensure the pair is unique so ON CONFLICT (pokemon_id, move_id) works
ALTER TABLE pokemon_moves
    ADD CONSTRAINT pokemon_moves_unique UNIQUE (pokemon_id, move_id);

-- (optional but recommended) FK + simple indexes for lookups
-- Adjust table/column names if different in your schema
ALTER TABLE pokemon_moves
    ADD CONSTRAINT fk_pokemon_moves_pokemon
        FOREIGN KEY (pokemon_id) REFERENCES pokedex(id)
        ON DELETE CASCADE;

ALTER TABLE pokemon_moves
    ADD CONSTRAINT fk_pokemon_moves_move
        FOREIGN KEY (move_id) REFERENCES moves(move_id)
        ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_pokemon_moves_pokemon_id ON pokemon_moves (pokemon_id);
CREATE INDEX IF NOT EXISTS idx_pokemon_moves_move_id ON pokemon_moves (move_id);


-- +goose Down
DROP INDEX IF EXISTS idx_pokemon_moves_move_id;
DROP INDEX IF EXISTS idx_pokemon_moves_pokemon_id;

ALTER TABLE pokemon_moves
    DROP CONSTRAINT IF EXISTS fk_pokemon_moves_move;

ALTER TABLE pokemon_moves
    DROP CONSTRAINT IF EXISTS fk_pokemon_moves_pokemon;

ALTER TABLE pokemon_moves
    DROP CONSTRAINT IF EXISTS pokemon_moves_unique;
