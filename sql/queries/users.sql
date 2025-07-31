-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserBySessionToken :one
SELECT * FROM users WHERE session_token = $1;

-- name: CreateUser :exec
INSERT INTO users (
    id,
    username,
    password_hash,
    created_at
) VALUES (
    $1, $2, $3, NOW()
);

-- name: SetUserSession :exec
UPDATE users
SET session_token = $1,
    csrf_token = $2
WHERE id = $3;
