-- name: CreateAuditLog :exec
INSERT INTO audit_logs (
  api_key_id, user_email, action, resource_type, resource_id,
  project_id, environment, before_state, after_state, changes,
  ip_address, user_agent, request_id, status, error_message,
  resource, details
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
);

-- name: ListAuditLogs :many
SELECT * FROM audit_logs 
WHERE 
  (sqlc.narg('project_id')::text IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('resource_type')::text IS NULL OR resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('resource_id')::text IS NULL OR resource_id = sqlc.narg('resource_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('start_date')::timestamptz IS NULL OR timestamp >= sqlc.narg('start_date'))
  AND (sqlc.narg('end_date')::timestamptz IS NULL OR timestamp <= sqlc.narg('end_date'))
ORDER BY timestamp DESC, id
LIMIT $1 OFFSET $2;

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_logs
WHERE 
  (sqlc.narg('project_id')::text IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('resource_type')::text IS NULL OR resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('resource_id')::text IS NULL OR resource_id = sqlc.narg('resource_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('start_date')::timestamptz IS NULL OR timestamp >= sqlc.narg('start_date'))
  AND (sqlc.narg('end_date')::timestamptz IS NULL OR timestamp <= sqlc.narg('end_date'));

-- name: GetAuditLogsByAPIKey :many
SELECT * FROM audit_logs
WHERE api_key_id = $1
ORDER BY timestamp DESC, id
LIMIT $2 OFFSET $3;
