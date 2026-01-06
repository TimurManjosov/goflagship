# Security Policy

## Supported Versions

The following versions of goflagship are currently supported with security updates:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| < 1.0   | :x:                |

**Note:** This project is currently in active development (pre-1.0). Security updates are applied to the `main` branch.

---

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability, please report it privately to ensure responsible disclosure:

### How to Report

1. **Email:** Send details to **[timur.manjosov@example.com]** (replace with actual contact)
   - Subject line: `[SECURITY] goflagship vulnerability report`
   
2. **GitHub Security Advisories:** Use the "Report a vulnerability" button in the Security tab

### What to Include

When reporting a vulnerability, please include:

- **Description** of the vulnerability
- **Steps to reproduce** the issue
- **Potential impact** and attack scenarios
- **Affected versions** or configurations
- **Suggested fix** (if you have one)
- **Your contact information** for follow-up

### Response Timeline

- **Initial Response:** Within 48 hours
- **Status Update:** Within 7 days
- **Fix Timeline:** Depends on severity
  - Critical: 24-48 hours
  - High: 1 week
  - Medium: 2 weeks
  - Low: Next release cycle

### After Reporting

1. We will **acknowledge** your report within 48 hours
2. We will **investigate** and provide a status update
3. We will **develop and test** a fix
4. We will **coordinate disclosure** timing with you
5. We will **credit you** in the release notes (unless you prefer to remain anonymous)

---

## Security Best Practices

### Deployment Security

#### 1. API Key Management

**DO:**
- ✅ Use strong, randomly generated API keys (32+ characters)
- ✅ Store keys in environment variables or secrets management (not in code)
- ✅ Rotate keys regularly (at least quarterly)
- ✅ Use different keys for different environments (dev, staging, prod)
- ✅ Set expiry dates for API keys when creating them

**DON'T:**
- ❌ Commit API keys to version control
- ❌ Share keys via email, Slack, or insecure channels
- ❌ Use default keys (`admin-123`) in production
- ❌ Reuse the same key across environments

#### 2. Database Security

**DO:**
- ✅ Use strong database passwords
- ✅ Enable SSL/TLS for database connections (`sslmode=require` in production)
- ✅ Restrict database access to application servers only
- ✅ Use connection pooling with limits
- ✅ Run database migrations in controlled manner

**DON'T:**
- ❌ Expose PostgreSQL directly to the internet
- ❌ Use default credentials
- ❌ Grant excessive database privileges to application user

#### 3. Network Security

**DO:**
- ✅ Run behind a reverse proxy (nginx, Caddy) with TLS
- ✅ Use HTTPS in production
- ✅ Configure CORS appropriately (don't use `*` in production)
- ✅ Enable rate limiting (configured via env vars)
- ✅ Use firewall rules to restrict access

**DON'T:**
- ❌ Expose metrics/pprof endpoints (`:9090`) publicly
- ❌ Run without TLS in production
- ❌ Allow unrestricted CORS origins

#### 4. Rollout Salt Security

**DO:**
- ✅ Set a stable `ROLLOUT_SALT` in production
- ✅ Keep the salt secret (treat like a password)
- ✅ Document the salt value securely (for disaster recovery)
- ✅ Use a long, random string (16+ bytes, hex-encoded)

**DON'T:**
- ❌ Change the salt frequently (it changes user bucketing)
- ❌ Share the salt publicly
- ❌ Use predictable values

### Configuration Security

#### Environment Variables

Sensitive environment variables that should be protected:

- `ADMIN_API_KEY` - Administrative API key
- `CLIENT_API_KEY` - Client API key (if using legacy auth)
- `DB_DSN` - Database connection string (contains password)
- `ROLLOUT_SALT` - Rollout bucketing salt

**Storage recommendations:**
- Use `.env` files locally (add to `.gitignore`)
- Use secrets managers in production (AWS Secrets Manager, HashiCorp Vault, etc.)
- Use Kubernetes secrets for containerized deployments
- Use platform-specific secrets (Heroku config vars, etc.)

#### Sample Secure Configuration

```bash
# .env (for local development only)
DB_DSN=postgres://user:STRONG_PASSWORD@localhost:5432/flagship?sslmode=require
ADMIN_API_KEY=$(openssl rand -hex 32)
CLIENT_API_KEY=$(openssl rand -hex 32)
ROLLOUT_SALT=$(openssl rand -hex 16)
ENV=prod
APP_HTTP_ADDR=:8080
METRICS_ADDR=127.0.0.1:9090  # Only bind metrics to localhost
```

### Authentication & Authorization

#### Role-Based Access Control (RBAC)

goflagship supports three roles:

- **readonly** - Can read flags, cannot modify
- **admin** - Can create, update, and delete flags
- **superadmin** - Full access including API key management

**Best Practices:**
- Use principle of least privilege
- Create separate API keys for different services
- Use `readonly` keys for production services that only need to read flags
- Restrict `superadmin` access to a few trusted humans
- Enable audit logging to track who changed what

#### API Key Security

API keys are stored hashed (bcrypt) in the database. They are:
- Generated with cryptographically secure random bytes
- Prefixed with `fsk_` for easy identification
- Hashed before storage (never stored in plaintext)
- Support expiry dates
- Can be revoked at any time

See [AUTH_SETUP.md](AUTH_SETUP.md) for detailed authentication setup.

### Audit Logging

goflagship includes audit logging for all administrative operations:

- Flag creation, updates, and deletions
- API key creation and revocation
- Authentication attempts

**Recommendations:**
- Enable audit logging in production
- Review audit logs regularly
- Export logs to a SIEM or log aggregation service
- Set up alerts for suspicious activity

### Data Privacy

#### Personally Identifiable Information (PII)

- User IDs used for rollouts should be **opaque identifiers** (not emails or names)
- Flag configurations should not contain PII
- Audit logs include user identifiers but should be protected
- Consider data retention policies for audit logs

#### GDPR Compliance

If operating in the EU:
- User IDs should be pseudonymized or anonymized
- Provide mechanisms to purge user data on request
- Document data processing in your privacy policy
- Consider data residency requirements

---

## Known Security Considerations

### 1. In-Memory Store Mode

- ⚠️ **Not recommended for production**: Data is lost on restart
- ⚠️ No persistence means no audit trail
- ✅ Use PostgreSQL store for production deployments

### 2. SSE (Server-Sent Events)

- ⚠️ Long-lived connections can consume resources
- ✅ Rate limiting is enabled by default
- ✅ Connection limits should be set at reverse proxy

### 3. Rate Limiting

Default rate limits:
- 100 requests/min per IP (public endpoints)
- 1000 requests/min per API key (authenticated)
- 60 requests/min per API key (admin endpoints)

**Adjust based on your traffic:**
```bash
RATE_LIMIT_PER_IP=100
RATE_LIMIT_PER_KEY=1000
RATE_LIMIT_ADMIN_PER_KEY=60
```

### 4. Input Validation

- ✅ All inputs are validated (flag keys, environment names, rollout percentages)
- ✅ JSON configs are validated for structure
- ✅ Expressions are validated before storage
- ⚠️ Very large flag configs could consume memory (consider size limits)

---

## Security Checklist for Production

Before deploying to production, verify:

- [ ] Strong, unique API keys set (not defaults)
- [ ] Database uses strong password and SSL
- [ ] `ROLLOUT_SALT` is set and documented
- [ ] HTTPS/TLS enabled on all public endpoints
- [ ] Metrics endpoint (`:9090`) not exposed publicly
- [ ] CORS configured appropriately (not `*`)
- [ ] Rate limiting configured
- [ ] Audit logging enabled
- [ ] Database backups configured
- [ ] Secrets stored in secure secrets manager
- [ ] API keys rotated regularly
- [ ] Monitoring and alerting configured
- [ ] Security updates applied promptly

---

## Security Disclosures

Past security issues will be documented here once reported and fixed.

**No security issues have been reported yet.**

---

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go Security Best Practices](https://go.dev/doc/security/best-practices)
- [PostgreSQL Security](https://www.postgresql.org/docs/current/security.html)
- [Authentication Setup Guide](AUTH_SETUP.md)

---

## Questions?

If you have questions about security that are **not sensitive vulnerabilities**, you can:
- Open a GitHub discussion
- Ask in the community channels

For **sensitive security matters**, always use private reporting channels described above.

Thank you for helping keep goflagship secure! 🔒
