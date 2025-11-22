# Implement Auth and Security Hardening

## Problem / Motivation

The current authentication system has several security and operational limitations:

1. **Hardcoded Admin Key**: The `ADMIN_API_KEY` is a single shared secret stored in environment variables
2. **No Role Separation**: No distinction between read-only and admin operations
3. **Limited Rate Limiting**: Basic IP-based rate limiting doesn't protect against distributed attacks
4. **No Token Expiry**: Bearer tokens never expire
5. **No Audit Trail**: No logging of who performed admin actions
6. **Credential Rotation**: Changing admin key requires restart and affects all users

These issues make the system unsuitable for production multi-user deployments and compliance requirements.

## Proposed Solution

Implement a layered security approach:

1. **Externalized API Keys**: Store keys in database with metadata (name, created_at, last_used)
2. **Optional RBAC**: Support read-only vs admin roles
3. **Enhanced Rate Limiting**: Per-key rate limits in addition to IP-based limits
4. **Token Management**: Add key rotation, expiry, and revocation
5. **Audit Logging**: Log all authenticated actions with timestamp, key ID, and IP

## Concrete Tasks

### Phase 1: API Key Storage
- [ ] Create database migration for `api_keys` table:
  - `id` (UUID), `name` (text), `key_hash` (text)
  - `role` (enum: admin, readonly)
  - `enabled` (boolean), `expires_at` (timestamp nullable)
  - `created_at`, `last_used_at`, `created_by`
- [ ] Add sqlc queries for key CRUD operations
- [ ] Implement bcrypt/argon2 hashing for key storage
- [ ] Add admin endpoint `POST /v1/admin/keys` to create keys
- [ ] Add admin endpoint `GET /v1/admin/keys` to list keys (without revealing actual keys)
- [ ] Add admin endpoint `DELETE /v1/admin/keys/:id` to revoke keys

### Phase 2: Enhanced Auth Middleware
- [ ] Create `internal/auth/middleware.go` package
- [ ] Implement `AuthRequired(role string)` middleware
- [ ] Support both legacy `ADMIN_API_KEY` (for backward compatibility) and DB keys
- [ ] Add constant-time comparison for all key checks
- [ ] Track last_used_at timestamp on successful auth
- [ ] Return 401 for expired/revoked keys with clear message

### Phase 3: Role-Based Access Control
- [ ] Define roles enum: `readonly`, `admin`, `superadmin`
- [ ] Update auth middleware to check roles
- [ ] Protect admin endpoints (POST/DELETE) with `admin` role
- [ ] Allow snapshot/stream endpoints with `readonly` role
- [ ] Add role validation in key creation

### Phase 4: Rate Limiting Enhancements
- [ ] Implement per-key rate limiting (separate from IP limits)
- [ ] Add configurable limits via environment:
  - `RATE_LIMIT_PER_IP` (default: 100/min)
  - `RATE_LIMIT_PER_KEY` (default: 1000/min)
  - `RATE_LIMIT_ADMIN_PER_KEY` (default: 60/min)
- [ ] Return 429 with `Retry-After` header on limit exceeded
- [ ] Add rate limit metrics to Prometheus

### Phase 5: Audit Logging
- [ ] Create `audit_logs` table:
  - `id`, `timestamp`, `api_key_id`, `action`, `resource`, `ip_address`, `user_agent`, `status`
- [ ] Log all admin operations (flag create/update/delete)
- [ ] Log all key management operations
- [ ] Add admin endpoint `GET /v1/admin/audit-logs` (paginated)
- [ ] Consider log retention policy (auto-delete after 90 days)

### Phase 6: Documentation & Migration
- [ ] Update README with new auth setup instructions
- [ ] Create migration guide for existing deployments
- [ ] Add example key generation scripts
- [ ] Document role system and permissions matrix
- [ ] Add troubleshooting section for auth issues

## API Changes

### New Endpoints
```
POST   /v1/admin/keys          Create API key (superadmin only)
GET    /v1/admin/keys          List API keys (admin+)
DELETE /v1/admin/keys/:id      Revoke API key (superadmin only)
GET    /v1/admin/audit-logs    View audit logs (admin+)
```

### Request/Response Examples

**Create Key**
```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ci-pipeline",
    "role": "admin",
    "expires_at": "2025-12-31T23:59:59Z"
  }'

# Response
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "ci-pipeline",
  "key": "fsk_live_4d8f9c8e7a6b5c4d3e2f1a0b9c8d7e6f5",  # shown only once
  "role": "admin",
  "created_at": "2025-01-15T10:30:00Z",
  "expires_at": "2025-12-31T23:59:59Z"
}
```

**List Keys**
```bash
curl http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_API_KEY"

# Response
{
  "keys": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "ci-pipeline",
      "role": "admin",
      "enabled": true,
      "created_at": "2025-01-15T10:30:00Z",
      "last_used_at": "2025-01-16T14:22:00Z",
      "expires_at": "2025-12-31T23:59:59Z"
    }
  ]
}
```

### Environment Variables
```bash
# Backward compatible
ADMIN_API_KEY=admin-123  # Still works, treated as superadmin

# New optional settings
AUTH_TOKEN_PREFIX=fsk_    # For key format: fsk_live_xxx
RATE_LIMIT_PER_IP=100
RATE_LIMIT_PER_KEY=1000
RATE_LIMIT_ADMIN_PER_KEY=60
```

## Acceptance Criteria

- [ ] API keys can be created, listed, and revoked via admin endpoints
- [ ] Keys are stored as secure hashes (bcrypt/argon2), never plaintext
- [ ] Role-based access control works: readonly can only read, admin can write
- [ ] Legacy `ADMIN_API_KEY` environment variable still works for backward compatibility
- [ ] Per-key rate limiting prevents abuse from single compromised key
- [ ] All admin actions are logged to audit_logs table
- [ ] Expired keys are automatically rejected with clear error message
- [ ] 429 responses include `Retry-After` header
- [ ] Prometheus metrics track:
  - Active API keys count
  - Auth failures by reason
  - Rate limit hits
- [ ] Documentation includes:
  - Key generation examples
  - Role permission matrix
  - Migration guide from env var to DB keys

## Notes / Risks / Edge Cases

### Risks
- **Backward Compatibility**: Existing deployments rely on `ADMIN_API_KEY`
  - Mitigation: Keep env var support, treat it as superadmin role
- **Key Leakage**: Generated keys shown in API response could be logged
  - Mitigation: Warn in docs to avoid logging responses, use secure channels
- **Rate Limit Bypass**: Attackers could rotate keys to bypass per-key limits
  - Mitigation: Keep IP-based limits as primary defense
- **Audit Log Growth**: High-traffic systems could generate huge logs
  - Mitigation: Implement retention policy, consider partitioning

### Edge Cases
- What if superadmin key is lost? Need emergency recovery procedure
- Concurrent key creation with same name should be allowed (add unique index on name+created_at)
- Handle clock skew for expires_at validation
- Key rotation: what happens to in-flight requests?
- Rate limit reset timing (per minute, sliding window, or fixed window?)

### Security Best Practices
- Use argon2id or bcrypt with cost factor 12+
- Generate keys with crypto/rand, not math/rand
- Include key prefix (e.g., `fsk_`) to identify leaked keys in commits
- Support key scoping to specific environments in future
- Consider adding IP whitelist per key

### Future Enhancements
- OAuth2/OIDC integration for user-based auth
- JWT tokens with short expiry + refresh tokens
- SCIM integration for enterprise SSO
- Webhook signatures for event authenticity
- mTLS support for service-to-service auth

## Implementation Hints

- Current auth is in `internal/api/server.go`'s `authAdmin` middleware (lines 285-300)
- Rate limiting uses `chi/httprate` middleware
- Database migrations are in `internal/db/migrations/` using Goose
- sqlc queries are in `internal/db/queries/flags.sql`
- Consider creating `internal/auth/` package for auth logic
- Use `crypto/subtle.ConstantTimeCompare` for key comparisons (already used in codebase)
- Prometheus metrics setup is in `internal/telemetry/`

## Labels

`feature`, `backend`, `security`, `breaking-change` (if ADMIN_API_KEY removed), `high-priority`

## Estimated Effort

**3 days** (experienced developer)
- Day 1: Database schema + key CRUD endpoints + hashing
- Day 2: Enhanced middleware + RBAC + rate limiting
- Day 3: Audit logging + documentation + testing
