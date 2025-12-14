-- +goose Up
-- Enhance audit_logs table with comprehensive fields per issue requirements

-- Add new columns for comprehensive audit logging
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS resource_type TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS resource_id TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS project_id TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS environment TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS before_state JSONB;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS after_state JSONB;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS changes JSONB;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS request_id TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_email TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS error_message TEXT;

-- Update existing resource column to be nullable (will migrate to resource_type/resource_id)
ALTER TABLE audit_logs ALTER COLUMN resource DROP NOT NULL;

-- Create indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON audit_logs(resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_project_id ON audit_logs(project_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp_resource ON audit_logs(timestamp DESC, resource_type, resource_id);

-- +goose Down
-- Remove indexes
DROP INDEX IF EXISTS idx_audit_logs_timestamp_resource;
DROP INDEX IF EXISTS idx_audit_logs_request_id;
DROP INDEX IF EXISTS idx_audit_logs_project_id;
DROP INDEX IF EXISTS idx_audit_logs_resource_id;
DROP INDEX IF EXISTS idx_audit_logs_resource_type;

-- Remove new columns
ALTER TABLE audit_logs DROP COLUMN IF EXISTS error_message;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS user_email;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS request_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS changes;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS after_state;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS before_state;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS environment;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS project_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS resource_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS resource_type;

-- Restore NOT NULL constraint on resource
ALTER TABLE audit_logs ALTER COLUMN resource SET NOT NULL;
