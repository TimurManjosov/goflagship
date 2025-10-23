-- name: GetAllFlags :many
SELECT * FROM flags WHERE env = $1 ORDER BY key;

-- name: GetFlagByKey :one
SELECT * FROM flags WHERE key = $1;

-- name: UpsertFlag :exec
INSERT INTO flags (key, description, enabled, rollout, expression, config, env)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (key) DO UPDATE SET
  description = EXCLUDED.description,
  enabled     = EXCLUDED.enabled,
  rollout     = EXCLUDED.rollout,
  expression  = EXCLUDED.expression,
  config      = EXCLUDED.config,
  env         = EXCLUDED.env,
  updated_at  = now();
