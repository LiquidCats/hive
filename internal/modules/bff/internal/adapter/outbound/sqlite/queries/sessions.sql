-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?);

-- name: GetSessionUser :one
SELECT sqlc.embed(users), sessions.expires_at
FROM sessions JOIN users ON users.id = sessions.user_id
WHERE token_hash = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = ?;

-- name: DeleteSessionsByUser :exec
DELETE FROM sessions WHERE user_id = ?;

-- Called on create, like DeleteStaleCeremonies; Auth.Session enforces expiry itself.
-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < unixepoch();
