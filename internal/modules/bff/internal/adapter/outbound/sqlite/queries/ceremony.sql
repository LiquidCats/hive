-- name: CreateCeremony :exec
INSERT INTO ceremonies (id, challenge, type, username, session) VALUES (?, ?, ?, ?, ?);

-- name: TakeCeremonyByChallenge :one
DELETE FROM ceremonies WHERE challenge = ? RETURNING *;

-- Start endpoints are public: purge abandoned ceremonies so they cannot pile up.
-- 10 minutes outlives the 5 minute WebAuthn timeout, which is enforced from the session itself.
-- name: DeleteStaleCeremonies :exec
DELETE FROM ceremonies WHERE created_at < datetime('now', '-10 minutes');
