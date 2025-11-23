-- name: CreateAPIKey :one
INSERT INTO api_keys (name, key_hash, role, enabled, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAPIKeyByID :one
SELECT * FROM api_keys WHERE id = $1;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1 AND enabled = true;

-- name: ListAPIKeys :many
SELECT * FROM api_keys ORDER BY created_at DESC;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- name: RevokeAPIKey :exec
UPDATE api_keys SET enabled = false WHERE id = $1;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE id = $1;
