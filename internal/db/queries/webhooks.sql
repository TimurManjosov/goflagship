-- name: CreateWebhook :one
INSERT INTO webhooks (url, description, enabled, events, project_id, environments, secret, max_retries, timeout_seconds)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListWebhooks :many
SELECT * FROM webhooks ORDER BY created_at DESC;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: UpdateWebhook :exec
UPDATE webhooks SET 
  url = $2, 
  description = $3, 
  enabled = $4, 
  events = $5,
  project_id = $6, 
  environments = $7, 
  max_retries = $8,
  timeout_seconds = $9, 
  updated_at = now()
WHERE id = $1;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = $1;

-- name: GetActiveWebhooks :many
SELECT * FROM webhooks WHERE enabled = true ORDER BY created_at DESC;

-- name: UpdateWebhookLastTriggered :exec
UPDATE webhooks SET last_triggered_at = now() WHERE id = $1;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
  webhook_id, 
  event_type, 
  payload, 
  status_code, 
  response_body, 
  error_message, 
  duration_ms, 
  success, 
  retry_count
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries 
WHERE webhook_id = $1 
ORDER BY timestamp DESC 
LIMIT $2 OFFSET $3;

-- name: CountWebhookDeliveries :one
SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = $1;
