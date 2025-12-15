-- +goose Up
CREATE TABLE IF NOT EXISTS webhooks (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  url TEXT NOT NULL,
  description TEXT,
  enabled BOOLEAN NOT NULL DEFAULT true,
  
  -- Event filtering
  events TEXT[] NOT NULL,
  project_id UUID,
  environments TEXT[],
  
  -- Security
  secret TEXT NOT NULL,
  
  -- Retry configuration
  max_retries INTEGER NOT NULL DEFAULT 3,
  timeout_seconds INTEGER NOT NULL DEFAULT 10,
  
  -- Metadata
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_triggered_at TIMESTAMPTZ
);

CREATE INDEX idx_webhooks_enabled ON webhooks(enabled) WHERE enabled = true;
CREATE INDEX idx_webhooks_project ON webhooks(project_id) WHERE project_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  webhook_id UUID REFERENCES webhooks(id) ON DELETE CASCADE,
  
  -- Request
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  -- Response
  status_code INTEGER,
  response_body TEXT,
  error_message TEXT,
  
  -- Timing
  duration_ms INTEGER,
  
  -- Result
  success BOOLEAN NOT NULL,
  retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_timestamp ON webhook_deliveries(timestamp DESC);

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
