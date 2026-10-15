-- The first user is approved; later ones wait for approval. One statement, so two racing first sign-ups cannot both win.
-- name: CreateUser :one
INSERT INTO users (id, name, disabled_at)
VALUES (?, ?, CASE WHEN EXISTS (SELECT 1 FROM users) THEN CURRENT_TIMESTAMP END)
RETURNING *;

-- name: GetUserByName :one
SELECT * FROM users WHERE name = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at, name;

-- name: DisableUserByID :execrows
UPDATE users SET disabled_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: EnableUserByID :execrows
UPDATE users SET disabled_at = NULL WHERE id = ?;
