-- name: InsertPokedex :exec
INSERT INTO pokedex (
    id, name, type_1, type_2, hp, attack, defense, special_attack, special_defense, speed
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
);

-- name: FetchPokemonDataById :one
SELECT * FROM pokedex WHERE id = $1;

-- name: FetchPokemonDataByName :one
SELECT * FROM pokedex WHERE LOWER(name) = LOWER($1);
