-- +goose Up
CREATE TABLE pokedex (
    id INT PRIMARY KEY,
    name TEXT NOT NULL,
    type_1 TEXT NOT NULL,
    type_2 TEXT,
    hp INT NOT NULL,
    attack INT NOT NULL,
    defense INT NOT NULL,
    special_attack INT NOT NULL,
    special_defense INT NOT NULL,
    speed INT NOT NULL
);

CREATE TABLE pokemon_moves (
    id SERIAL PRIMARY KEY,
    pokemon_id INT REFERENCES pokedex(id) ON DELETE CASCADE,
    move_name TEXT NOT NULL,
    power INT NOT NULL
);

CREATE TABLE user_pokemon (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    pokemon_id INT REFERENCES pokedex(id),
    nickname TEXT,
    current_hp INT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT now(),
    
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX unique_active_pokemon_per_user
ON user_pokemon (user_id)
WHERE is_active = true;

CREATE TABLE challenger_pokemon (
    id UUID PRIMARY KEY,
    pokemon_id INT REFERENCES pokedex(id),
    current_hp INT NOT NULL,
    created_at TIMESTAMP DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS challenger_pokemon;

DROP INDEX IF EXISTS unique_active_pokemon_per_user;

DROP TABLE IF EXISTS user_pokemon;
DROP TABLE IF EXISTS pokemon_moves;
DROP TABLE IF EXISTS pokedex;
