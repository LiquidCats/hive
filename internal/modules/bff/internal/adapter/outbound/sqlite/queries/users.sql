-- The first user is the approved owner; later ones are users waiting for approval.
-- One statement, so two racing first sign-ups cannot both win.
-- name: CreateUser :one
INSERT INTO users (id, name, role, disabled_at)
VALUES (
    ?, ?,
    CASE WHEN EXISTS (SELECT 1 FROM users) THEN 'user' ELSE 'owner' END,
    CASE WHEN EXISTS (SELECT 1 FROM users) THEN CURRENT_TIMESTAMP END
)
RETURNING *;

-- name: GetUserByName :one
SELECT * FROM users WHERE name = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- Keyset page: IDs are UUIDv7, so id order is registration order. The first page passes the zero UUID.
-- name: ListUsers :many
SELECT * FROM users WHERE id > ? ORDER BY id LIMIT ?;

-- The page before an ID, nearest first: the caller reverses it to oldest first.
-- name: ListUsersBefore :many
SELECT * FROM users WHERE id < ? ORDER BY id DESC LIMIT ?;

-- Disabling also demotes, so re-enabling yields a user, never a returning admin.
-- The owner is never disabled: Users.DisableUser refuses the actor themselves, and only the owner may call it.
-- name: DisableUserByID :execrows
UPDATE users SET disabled_at = CURRENT_TIMESTAMP, role = 'user' WHERE id = ?;

-- name: EnableUserByID :execrows
UPDATE users SET disabled_at = NULL WHERE id = ?;

-- Active users only, so approving a pending user always yields a user.
-- name: SetUserRoleByID :execrows
UPDATE users SET role = ? WHERE id = ? AND disabled_at IS NULL;
