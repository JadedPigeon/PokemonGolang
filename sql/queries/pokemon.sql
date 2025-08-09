-- name: InsertPokedex :exec
INSERT INTO pokedex (
    id, name, type_1, type_2, hp, attack, defense, special_attack, special_defense, speed, image_url
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: FetchPokemonDataById :one
SELECT * FROM pokedex WHERE id = $1;

-- name: FetchPokemonDataByName :one
SELECT * FROM pokedex WHERE LOWER(name) = LOWER($1);

-- name: GetMoveByID :one
SELECT * FROM moves WHERE move_id = $1;

-- name: InsertMove :exec
INSERT INTO moves (move_id, name, power, type, description)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertPokemonMove :exec
INSERT INTO pokemon_moves (pokemon_id, move_id)
VALUES ($1, $2)
ON CONFLICT (pokemon_id, move_id) DO NOTHING;

-- name: InsertUserPokemon :exec
INSERT INTO user_pokemon (
    id,
    user_id,
    pokemon_id,
    nickname,
    current_hp,
    is_active,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, DEFAULT
);

-- name: CountUserPokemon :one
SELECT COUNT(*) FROM user_pokemon WHERE user_id = $1;

-- name: DeactivateAllUserPokemon :exec
UPDATE user_pokemon
SET is_active = false
WHERE user_id = $1;

-- name: ActivateUserPokemon :one
UPDATE user_pokemon
SET is_active = TRUE
WHERE user_id = $1 AND id = $2
RETURNING id;

-- name: InsertChallengePokemon :exec
INSERT INTO challenger_pokemon (
    id,
    pokemon_id,
    current_hp,
    created_at
) VALUES (
    $1, $2, $3, DEFAULT
);

-- name: SetUserChallengePokemon :exec
UPDATE users
SET challenge_pokemon_id = $1
WHERE id = $2;

-- name: GetUserChallengePokemon :one
SELECT cp.*
FROM users u
JOIN challenger_pokemon cp ON u.challenge_pokemon_id = cp.id
WHERE u.id = $1;

-- name: DeleteChallengePokemon :exec
DELETE FROM challenger_pokemon
WHERE id = $1;

-- name: GetAllUserPokemon :many
SELECT p.*, up.is_active
FROM user_pokemon up
JOIN pokedex p ON up.pokemon_id = p.id
WHERE up.user_id = $1;


-- name: GetOneUserPokemon :one
SELECT *
FROM user_pokemon
WHERE user_id = $1 and pokemon_id = $2;