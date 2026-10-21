-- name: CreateRepo :one
INSERT INTO repositories (id, name, url, username, password, ssh_private_key, insecure)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetRepoByID :one
SELECT * FROM repositories WHERE id = ?;

-- Keyset page: IDs are UUIDv7, so id order is creation order. The first page passes the zero UUID.
-- name: ListRepos :many
SELECT * FROM repositories WHERE id > ? ORDER BY id LIMIT ?;

-- name: UpdateRepo :one
UPDATE repositories
SET name = ?, url = ?, username = ?, password = ?, ssh_private_key = ?, insecure = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteRepoByID :execrows
DELETE FROM repositories WHERE id = ?;
