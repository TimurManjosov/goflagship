-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS flags (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  key TEXT UNIQUE NOT NULL,
  description TEXT DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT false,
  rollout INTEGER NOT NULL DEFAULT 0,
  expression TEXT,
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  env TEXT NOT NULL DEFAULT 'prod',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS flags;
