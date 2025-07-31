-- +goose Up

-- Create moves table with metadata
CREATE TABLE moves (
    move_id INT PRIMARY KEY,
    name TEXT NOT NULL,
    power INT NOT NULL,
    type TEXT NOT NULL,
    description TEXT
);

-- Refactor pokemon_moves to be a join table
DROP TABLE IF EXISTS pokemon_moves;

CREATE TABLE pokemon_moves (
    id SERIAL PRIMARY KEY,
    pokemon_id INT REFERENCES pokedex(id) ON DELETE CASCADE,
    move_id INT REFERENCES moves(move_id) ON DELETE CASCADE
);

-- +goose Down

-- Drop the join table and metadata table
DROP TABLE IF EXISTS pokemon_moves;
DROP TABLE IF EXISTS moves;
