-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;
