# Implement Backend Audit Log

## Problem / Motivation

Currently, there is no record of who changed what and when. This creates problems for:

1. **Accountability**: Can't track which user/key made a change
2. **Debugging**: When a flag breaks production, can't find who/what changed
3. **Compliance**: Regulations (SOC 2, GDPR, HIPAA) require audit trails
4. **Security**: Can't detect unauthorized access or suspicious activity
5. **Rollback**: No easy way to see previous state to revert changes

Real-world scenarios:
- "Production is down, who changed the payment_enabled flag?"
- "Why was this flag disabled? We need to undo it."
- "Compliance audit: show all flag changes in Q4 2024"
- "Security incident: list all API key usage in last 24 hours"

## Proposed Solution

Implement a comprehensive audit logging system that records:

1. **All Mutations**: Flag create/update/delete, API key management, project changes
2. **Metadata**: Timestamp, user/key ID, IP address, user agent
3. **Before/After State**: What changed (diff)
4. **Query API**: Searchable, filterable, paginated audit logs

## Concrete Tasks

### Phase 1: Database Schema
- [ ] Create `audit_logs` table:
  ```sql
  CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- Who performed the action
    api_key_id UUID REFERENCES api_keys(id),  -- If using API key auth
    user_email TEXT,                          -- If using user auth
    
    -- What was done
    action TEXT NOT NULL,         -- created, updated, deleted
    resource_type TEXT NOT NULL,  -- flag, project, api_key
    resource_id TEXT NOT NULL,    -- flag key, project key, api key id
    
    -- Context
    project_id UUID REFERENCES projects(id),  -- If project-scoped
    environment TEXT,                         -- dev, staging, prod
    
    -- Details
    before_state JSONB,  -- Previous state (null for create)
    after_state JSONB,   -- New state (null for delete)
    changes JSONB,       -- Specific fields changed
    
    -- Request metadata
    ip_address INET,
    user_agent TEXT,
    request_id TEXT,     -- Correlate with request logs
    
    -- Result
    status TEXT,         -- success, failure
    error_message TEXT   -- If status = failure
  );
  
  -- Indexes for common queries
  CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
  CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
  CREATE INDEX idx_audit_logs_api_key ON audit_logs(api_key_id);
  CREATE INDEX idx_audit_logs_project ON audit_logs(project_id);
  ```
- [ ] Create migration: `202501170001_create_audit_logs.sql`
- [ ] Add retention policy (optional):
  ```sql
  -- Auto-delete logs older than 90 days
  CREATE OR REPLACE FUNCTION delete_old_audit_logs()
  RETURNS void AS $$
  BEGIN
    DELETE FROM audit_logs WHERE timestamp < now() - interval '90 days';
  END;
  $$ LANGUAGE plpgsql;
  ```

### Phase 2: Audit Logging Middleware
- [ ] Create `internal/audit/logger.go`:
  ```go
  type AuditLogger struct {
    repo *repo.Repo
  }
  
  type AuditEntry struct {
    Action       string
    ResourceType string
    ResourceID   string
    ProjectID    *string
    Environment  *string
    BeforeState  map[string]any
    AfterState   map[string]any
    Changes      map[string]any
    IPAddress    string
    UserAgent    string
    RequestID    string
    APIKeyID     *string
    Status       string
    ErrorMessage *string
  }
  
  func (a *AuditLogger) Log(ctx context.Context, entry AuditEntry) error
  ```
- [ ] Implement helper to compute diff:
  ```go
  // ComputeChanges returns map of changed fields
  func ComputeChanges(before, after map[string]any) map[string]any {
    changes := make(map[string]any)
    for key, newVal := range after {
      oldVal, exists := before[key]
      if !exists || !reflect.DeepEqual(oldVal, newVal) {
        changes[key] = map[string]any{
          "before": oldVal,
          "after":  newVal,
        }
      }
    }
    return changes
  }
  ```
- [ ] Integrate into handlers:
  ```go
  func (s *Server) handleUpsertFlag(w http.ResponseWriter, r *http.Request) {
    // ... existing logic ...
    
    // Before mutation, capture old state
    oldFlag, _ := s.repo.GetFlagByKey(ctx, req.Key)
    
    // Perform mutation
    err := s.repo.UpsertFlag(ctx, params)
    
    // After mutation, capture new state
    newFlag, _ := s.repo.GetFlagByKey(ctx, req.Key)
    
    // Log audit entry
    s.auditLogger.Log(ctx, audit.AuditEntry{
      Action:       "updated",  // or "created" if oldFlag was nil
      ResourceType: "flag",
      ResourceID:   req.Key,
      ProjectID:    &projectID,
      Environment:  &req.Env,
      BeforeState:  flagToMap(oldFlag),
      AfterState:   flagToMap(newFlag),
      Changes:      audit.ComputeChanges(oldState, newState),
      IPAddress:    r.RemoteAddr,
      UserAgent:    r.Header.Get("User-Agent"),
      RequestID:    middleware.GetReqID(ctx),
      APIKeyID:     getAPIKeyID(ctx),  // From auth middleware
      Status:       "success",
    })
  }
  ```

### Phase 3: Audit Query API
- [ ] Create sqlc queries in `internal/db/queries/audit.sql`:
  ```sql
  -- name: CreateAuditLog :one
  INSERT INTO audit_logs (
    api_key_id, user_email, action, resource_type, resource_id,
    project_id, environment, before_state, after_state, changes,
    ip_address, user_agent, request_id, status, error_message
  ) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
  ) RETURNING *;
  
  -- name: ListAuditLogs :many
  SELECT * FROM audit_logs
  WHERE 
    ($1::uuid IS NULL OR project_id = $1)
    AND ($2::text IS NULL OR resource_type = $2)
    AND ($3::text IS NULL OR resource_id = $3)
    AND ($4::timestamptz IS NULL OR timestamp >= $4)
    AND ($5::timestamptz IS NULL OR timestamp <= $5)
  ORDER BY timestamp DESC
  LIMIT $6 OFFSET $7;
  
  -- name: GetAuditLog :one
  SELECT * FROM audit_logs WHERE id = $1;
  
  -- name: CountAuditLogs :one
  SELECT COUNT(*) FROM audit_logs
  WHERE 
    ($1::uuid IS NULL OR project_id = $1)
    AND ($2::text IS NULL OR resource_type = $2);
  ```
- [ ] Create REST endpoint `GET /v1/admin/audit-logs`:
  ```go
  func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    projectID := r.URL.Query().Get("projectId")
    resourceType := r.URL.Query().Get("resourceType")  // flag, project, api_key
    resourceID := r.URL.Query().Get("resourceId")
    startDate := r.URL.Query().Get("startDate")        // ISO 8601
    endDate := r.URL.Query().Get("endDate")
    page := parseInt(r.URL.Query().Get("page"), 1)
    limit := parseInt(r.URL.Query().Get("limit"), 20)
    
    // Query database
    logs, err := s.repo.ListAuditLogs(ctx, filters, limit, offset)
    total, _ := s.repo.CountAuditLogs(ctx, filters)
    
    // Response
    writeJSON(w, 200, map[string]any{
      "logs": logs,
      "pagination": map[string]any{
        "page": page,
        "limit": limit,
        "total": total,
        "pages": (total + limit - 1) / limit,
      },
    })
  }
  ```

### Phase 4: Audit for All Mutations
- [ ] Add audit logging to all mutation endpoints:
  - `POST /v1/flags` (create/update flag)
  - `DELETE /v1/flags` (delete flag)
  - `POST /v1/projects` (create project)
  - `PUT /v1/projects/:key` (update project)
  - `DELETE /v1/projects/:key` (delete project)
  - `POST /v1/admin/keys` (create API key)
  - `DELETE /v1/admin/keys/:id` (revoke API key)
- [ ] Include read operations (optional, can be noisy):
  - `GET /v1/flags/snapshot` (flag access)
  - `GET /v1/flags/evaluate` (flag evaluation)
- [ ] Log authentication failures:
  - Invalid API key
  - Expired token
  - Insufficient permissions

### Phase 5: Diff Computation
- [ ] Implement smart diff for nested objects:
  ```go
  // Example: config changed from {"color": "red"} to {"color": "blue", "size": "large"}
  // Result: {
  //   "config.color": {"before": "red", "after": "blue"},
  //   "config.size": {"before": null, "after": "large"}
  // }
  ```
- [ ] Use JSON patch format (RFC 6902) for complex changes:
  ```json
  {
    "changes": [
      {"op": "replace", "path": "/config/color", "value": "blue"},
      {"op": "add", "path": "/config/size", "value": "large"}
    ]
  }
  ```
- [ ] Unit test diff computation with various inputs

### Phase 6: Export & Retention
- [ ] Add CSV export endpoint:
  ```
  GET /v1/admin/audit-logs/export?format=csv&startDate=2025-01-01
  ```
- [ ] Implement retention policy:
  - Environment variable: `AUDIT_LOG_RETENTION_DAYS=90`
  - Cron job or periodic task to delete old logs
  - Option to archive to S3/cloud storage before deletion
- [ ] Document GDPR compliance (anonymize user data on request)

### Phase 7: Testing & Documentation
- [ ] Unit tests for audit logger
- [ ] Integration tests for audit endpoints
- [ ] Test audit logging doesn't break on errors (non-blocking)
- [ ] Document audit log schema
- [ ] Document query API with examples
- [ ] Add audit log section to README

## API Changes

### New Endpoint

**GET /v1/admin/audit-logs**

Query Parameters:
- `projectId` (optional) - Filter by project
- `resourceType` (optional) - Filter by type: flag, project, api_key
- `resourceId` (optional) - Filter by specific resource
- `action` (optional) - Filter by action: created, updated, deleted
- `startDate` (optional) - ISO 8601 timestamp
- `endDate` (optional) - ISO 8601 timestamp
- `page` (default: 1)
- `limit` (default: 20, max: 100)

Example Request:
```bash
curl "http://localhost:8080/v1/admin/audit-logs?projectId=customer-a&resourceType=flag&page=1&limit=20" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

Example Response:
```json
{
  "logs": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "timestamp": "2025-01-15T10:30:00Z",
      "action": "updated",
      "resourceType": "flag",
      "resourceId": "feature_x",
      "projectId": "customer-a",
      "environment": "prod",
      "beforeState": {
        "enabled": true,
        "rollout": 50
      },
      "afterState": {
        "enabled": false,
        "rollout": 50
      },
      "changes": {
        "enabled": {
          "before": true,
          "after": false
        }
      },
      "ipAddress": "192.168.1.100",
      "userAgent": "Mozilla/5.0...",
      "requestId": "req-123",
      "apiKeyId": "key-456",
      "status": "success"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 147,
    "pages": 8
  }
}
```

**GET /v1/admin/audit-logs/export**

Query Parameters: Same as above, plus:
- `format` (required) - csv, json, jsonl

Example:
```bash
curl "http://localhost:8080/v1/admin/audit-logs/export?format=csv&startDate=2025-01-01" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -o audit-logs.csv
```

## Acceptance Criteria

### Database
- [ ] `audit_logs` table created with proper schema
- [ ] Indexes on frequently queried columns
- [ ] Foreign keys to projects and api_keys (with ON DELETE SET NULL)

### Audit Logging
- [ ] All flag mutations are logged (create, update, delete)
- [ ] All project mutations are logged (if Issue #11 implemented)
- [ ] All API key mutations are logged (if Issue #2 implemented)
- [ ] Authentication failures are logged
- [ ] Logs include before/after state
- [ ] Logs include diff (what changed)
- [ ] Logs include request metadata (IP, user agent, request ID)
- [ ] Audit logging is non-blocking (doesn't fail main request)

### Query API
- [ ] `GET /v1/admin/audit-logs` returns paginated results
- [ ] Filters work (project, resource, date range)
- [ ] Response includes pagination metadata
- [ ] Export to CSV works
- [ ] Export to JSON works

### Performance
- [ ] Audit logging adds <5ms overhead per request
- [ ] Querying 10k logs takes <500ms
- [ ] Indexes prevent full table scans

### Documentation
- [ ] Audit log schema documented
- [ ] Query API documented with examples
- [ ] Retention policy documented
- [ ] Compliance implications explained (GDPR, SOC 2)

## Notes / Risks / Edge Cases

### Risks
- **Performance**: Logging every request could slow down API
  - Mitigation: Async logging (write to queue, process in background)
  - Mitigation: Only log mutations, not reads (configurable)
- **Storage Growth**: Audit logs grow indefinitely
  - Mitigation: Retention policy, auto-delete old logs
  - Mitigation: Archive to cold storage (S3, Glacier)
- **Privacy**: Logs might contain PII (user IDs, emails)
  - Mitigation: Hash sensitive fields, support anonymization
- **Reliability**: Audit log failure shouldn't break main operation
  - Mitigation: Log errors separately, don't fail request

### Edge Cases
- Audit log insert fails (DB down) → log to stderr, don't fail request
- Before state unavailable (flag doesn't exist) → before_state = null
- Very large config (>1MB) → truncate or store as separate file
- Concurrent updates to same flag → both logged with timestamps
- API key deleted → audit logs reference deleted key (foreign key NULL)
- System actions (auto-expiry) vs user actions → distinguish with api_key_id = null

### Audit Log Levels (Future)

**Level 1: Mutations Only** (Recommended default)
- Log create/update/delete operations
- Low overhead, high value

**Level 2: All Authenticated Requests**
- Log all API calls (including reads)
- Higher overhead, useful for security audits

**Level 3: Everything**
- Log unauthenticated requests, errors, etc.
- Very high overhead, only for debugging

### Compliance Considerations

**GDPR (Right to be Forgotten)**
- User requests deletion → anonymize audit logs (replace email/ID with "anonymized")
- Keep audit trail but remove PII

**SOC 2 (Security Audit)**
- Logs must be immutable (append-only)
- Consider separate audit database with restricted access
- Regular log review and alerting

**HIPAA (Healthcare)**
- Extra sensitivity for audit logs
- Encrypted at rest and in transit
- Access controls (only admins can view logs)

### Future Enhancements
- Real-time audit log streaming (SSE feed)
- Slack/email notifications on specific actions (flag deleted, etc.)
- Anomaly detection (unusual activity patterns)
- Audit log dashboard (visualize changes over time)
- Rollback from audit log (undo changes)
- Compare snapshots at different timestamps
- Audit log replay (see system state at any point in time)

## Implementation Hints

- Current mutation handlers are in `internal/api/server.go`
- Use `chi/middleware.GetReqID()` to get request ID
- Store API key ID in request context during auth middleware
- Example async logging pattern:
  ```go
  type AuditLogger struct {
    queue chan AuditEntry
    repo  *repo.Repo
  }
  
  func (a *AuditLogger) Start() {
    go func() {
      for entry := range a.queue {
        _ = a.repo.CreateAuditLog(context.Background(), entry)
      }
    }()
  }
  
  func (a *AuditLogger) Log(entry AuditEntry) {
    select {
    case a.queue <- entry:
    default:
      log.Println("audit queue full, dropping log")
    }
  }
  ```
- JSON diff library: `github.com/nsf/jsondiff` or `github.com/wI2L/jsondiff`
- Consider using database triggers for guaranteed audit (even if app fails)

## Labels

`feature`, `backend`, `security`, `compliance`, `database`

## Estimated Effort

**2-3 days**
- Day 1: Database schema + audit logger + integration
- Day 2: Query API + filtering + pagination
- Day 3: Export, retention, testing, documentation
