-- name: CreateCredential :exec
INSERT INTO user_credentials (id, user_id, credential_id, credential) VALUES (?, ?, ?, ?);

-- name: ListActiveCredentialsByUser :many
SELECT * FROM user_credentials WHERE user_id = ? AND revoked_at IS NULL;

-- name: UpdateCredential :exec
UPDATE user_credentials SET credential = ? WHERE credential_id = ?;

-- name: RevokeCredentialByID :exec
UPDATE user_credentials SET revoked_at = CURRENT_TIMESTAMP WHERE id = ?;
