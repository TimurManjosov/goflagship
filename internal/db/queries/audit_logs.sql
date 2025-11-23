-- name: CreateAuditLog :exec
INSERT INTO audit_logs (api_key_id, action, resource, ip_address, user_agent, status, details)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAuditLogs :many
SELECT * FROM audit_logs 
ORDER BY timestamp DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_logs;

-- name: GetAuditLogsByAPIKey :many
SELECT * FROM audit_logs
WHERE api_key_id = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;
