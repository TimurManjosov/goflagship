-- +goose Up
CREATE TYPE api_key_role AS ENUM ('readonly', 'admin', 'superadmin');

CREATE TABLE IF NOT EXISTS api_keys (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL,
  role api_key_role NOT NULL DEFAULT 'readonly',
  enabled BOOLEAN NOT NULL DEFAULT true,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ,
  created_by TEXT NOT NULL DEFAULT 'system'
);

CREATE INDEX idx_api_keys_enabled ON api_keys(enabled) WHERE enabled = true;
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS api_keys;
DROP TYPE IF EXISTS api_key_role;
