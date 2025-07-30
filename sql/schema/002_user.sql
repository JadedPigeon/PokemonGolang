-- +goose Up
ALTER TABLE users
ADD COLUMN session_token TEXT,
ADD COLUMN csrf_token TEXT;

-- +goose Down
ALTER TABLE users
DROP COLUMN session_token,
DROP COLUMN csrf_token;
