# Authentication and Security Setup Guide

This guide explains how to set up and use the enhanced authentication and security features in go-flagship.

## Overview

go-flagship now supports:

- **Database-backed API Keys**: Store API keys securely in PostgreSQL with bcrypt hashing
- **Role-Based Access Control (RBAC)**: Three role levels (readonly, admin, superadmin)
- **Audit Logging**: Track all admin operations with timestamps, IP addresses, and user agents
- **Key Expiry and Revocation**: Set expiration dates and revoke keys when needed
- **Backward Compatibility**: Legacy `ADMIN_API_KEY` environment variable still works

## Quick Start

### 1. Run Database Migrations

The auth system requires two new database tables:

```bash
goose -dir internal/db/migrations postgres "$DB_DSN" up
```

This creates:
- `api_keys` table: Stores API key metadata and hashes
- `audit_logs` table: Stores audit trail of all admin operations

### 2. Create Your First API Key

Use the legacy admin key to create a new API key:

```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer admin-123" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-admin",
    "role": "admin",
    "expires_at": "2025-12-31T23:59:59Z"
  }'
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "production-admin",
  "key": "fsk_eJx8y9k3...",
  "role": "admin",
  "created_at": "2025-01-15T10:30:00Z",
  "expires_at": "2025-12-31T23:59:59Z"
}
```

**⚠️ Important**: The `key` field is only shown once! Store it securely.

### 3. Use the New API Key

```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer fsk_eJx8y9k3..." \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new_feature",
    "enabled": true,
    "env": "prod"
  }'
```

## API Endpoints

### Key Management

| Method | Endpoint | Role Required | Description |
|--------|----------|---------------|-------------|
| POST | `/v1/admin/keys` | superadmin | Create new API key |
| GET | `/v1/admin/keys` | admin+ | List all API keys |
| DELETE | `/v1/admin/keys/:id` | superadmin | Revoke API key |

### Audit Logs

| Method | Endpoint | Role Required | Description |
|--------|----------|---------------|-------------|
| GET | `/v1/admin/audit-logs` | admin+ | List audit logs (paginated) |

Query parameters for audit logs:
- `limit`: Number of results per page (default: 50, max: 100)
- `offset`: Pagination offset (default: 0)

Example:
```bash
curl http://localhost:8080/v1/admin/audit-logs?limit=20&offset=0 \
  -H "Authorization: Bearer fsk_eJx8y9k3..."
```

## Roles and Permissions

### Role Hierarchy

| Role | Can Read Flags | Can Write Flags | Can Manage Keys | Can View Audit Logs |
|------|----------------|-----------------|-----------------|---------------------|
| `readonly` | ✅ | ❌ | ❌ | ❌ |
| `admin` | ✅ | ✅ | ❌ | ✅ |
| `superadmin` | ✅ | ✅ | ✅ | ✅ |

### Permission Matrix

| Operation | readonly | admin | superadmin |
|-----------|----------|-------|------------|
| GET `/v1/flags/snapshot` | ✅ | ✅ | ✅ |
| GET `/v1/flags/stream` | ✅ | ✅ | ✅ |
| POST `/v1/flags` | ❌ | ✅ | ✅ |
| DELETE `/v1/flags` | ❌ | ✅ | ✅ |
| POST `/v1/admin/keys` | ❌ | ❌ | ✅ |
| GET `/v1/admin/keys` | ❌ | ✅ | ✅ |
| DELETE `/v1/admin/keys/:id` | ❌ | ❌ | ✅ |
| GET `/v1/admin/audit-logs` | ❌ | ✅ | ✅ |

## Environment Variables

```bash
# Legacy admin key (still works, treated as superadmin)
ADMIN_API_KEY=admin-123

# Optional: Customize API key prefix
AUTH_TOKEN_PREFIX=fsk_

# Rate limiting (requests per minute)
RATE_LIMIT_PER_IP=100
RATE_LIMIT_PER_KEY=1000
RATE_LIMIT_ADMIN_PER_KEY=60
```

## API Key Format

Generated API keys have the format:
```
fsk_<base64-encoded-random-bytes>
```

Example: `fsk_eJx8y9k3mQ7pL2vN5wR8tA`

The prefix `fsk_` (configurable via `AUTH_TOKEN_PREFIX`) helps identify leaked keys in version control systems.

## Security Best Practices

### 1. Key Storage
- **Never commit API keys** to version control
- Store keys in secure secrets management systems (e.g., HashiCorp Vault, AWS Secrets Manager)
- Use environment variables or secret files with restricted permissions

### 2. Key Rotation
```bash
# 1. Create new key
NEW_KEY=$(curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer $OLD_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"rotated-key","role":"admin"}' | jq -r '.key')

# 2. Update applications to use new key
# ... deploy changes ...

# 3. Revoke old key
curl -X DELETE http://localhost:8080/v1/admin/keys/$OLD_KEY_ID \
  -H "Authorization: Bearer $NEW_KEY"
```

### 3. Key Expiry
Set expiration dates on keys:
```json
{
  "name": "temporary-access",
  "role": "readonly",
  "expires_at": "2025-12-31T23:59:59Z"
}
```

Expired keys are automatically rejected with status 401.

### 4. Least Privilege
- Use `readonly` role for monitoring and reporting tools
- Use `admin` role for CI/CD pipelines that manage flags
- Reserve `superadmin` role for key management and security operations

## Audit Logging

All admin operations are logged with:
- **Timestamp**: When the action occurred
- **API Key ID**: Which key performed the action
- **Action**: What was done (e.g., `create_api_key`, `upsert_flag`)
- **Resource**: What was affected (e.g., `flags/prod/feature_x`)
- **IP Address**: Source IP
- **User Agent**: Client information
- **Status**: HTTP status code

Example audit log entry:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "timestamp": "2025-01-15T10:30:00Z",
  "api_key_id": "550e8400-e29b-41d4-a716-446655440000",
  "action": "upsert_flag",
  "resource": "flags/prod/new_feature",
  "ip_address": "192.168.1.100",
  "user_agent": "curl/7.68.0",
  "status": 200
}
```

## Migration Guide

### Migrating from Legacy ADMIN_API_KEY

1. **Keep using legacy key initially**: No changes required immediately
2. **Create database-backed keys**: Use the legacy key to create new keys
3. **Update applications**: Gradually migrate to new keys
4. **Monitor audit logs**: Verify all systems are using new keys
5. **Remove legacy key** (optional): Once migration is complete, you can remove the `ADMIN_API_KEY` environment variable

## Monitoring

### Prometheus Metrics

The system exports the following metrics:

```
# Number of active (enabled and non-expired) API keys
active_api_keys

# Total authentication failures by reason
auth_failures_total{reason="missing_token"}
auth_failures_total{reason="invalid_token"}
auth_failures_total{reason="expired_token"}
auth_failures_total{reason="disabled_key"}

# Total rate limit hits by type
rate_limit_hits_total{type="ip"}
rate_limit_hits_total{type="key"}
```

Access metrics at: `http://localhost:9090/metrics`

## Troubleshooting

### "invalid token" error
- Check that the API key is correct and hasn't been revoked
- Verify the key hasn't expired
- Ensure you're using the `Bearer` authentication scheme

### "insufficient permissions" error
- Check that your key's role has permission for the operation
- See the Permission Matrix above

### Keys not working after database migration
- Verify migrations ran successfully: `goose -dir internal/db/migrations postgres "$DB_DSN" status`
- Check database connection: `psql $DB_DSN -c "SELECT COUNT(*) FROM api_keys;"`

### High rate limit hits
- Check the `rate_limit_hits_total` metric
- Consider increasing `RATE_LIMIT_PER_KEY` for high-traffic services
- Implement exponential backoff in clients

## Examples

### Creating a Read-Only Key for Monitoring
```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "datadog-monitoring",
    "role": "readonly"
  }'
```

### Creating a Temporary Admin Key
```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "contractor-access",
    "role": "admin",
    "expires_at": "2025-02-01T00:00:00Z"
  }'
```

### Listing All Keys
```bash
curl http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY"
```

### Revoking a Compromised Key
```bash
curl -X DELETE http://localhost:8080/v1/admin/keys/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $ADMIN_KEY"
```

### Viewing Recent Admin Actions
```bash
curl "http://localhost:8080/v1/admin/audit-logs?limit=10" \
  -H "Authorization: Bearer $ADMIN_KEY"
```
