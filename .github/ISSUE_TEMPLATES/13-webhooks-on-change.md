# Implement Webhooks on Flag Changes

## Problem / Motivation

Currently, there's no way to notify external systems when flags change. Teams need to:

1. **Trigger CI/CD**: Redeploy when certain flags change
2. **Notify Teams**: Send Slack/Discord message on flag updates
3. **Sync Systems**: Update external caches, CDNs, or databases
4. **Analytics**: Track flag changes in external analytics platforms
5. **Compliance**: Forward audit events to SIEM systems

Real-world scenarios:
- "When `maintenance_mode` is enabled, send Slack alert to on-call team"
- "When any prod flag changes, trigger deployment notification in Datadog"
- "When experiment flags change, log to analytics platform"
- "Forward all flag events to enterprise audit system"

## Proposed Solution

Implement a webhook system that:

1. **Registers Webhook URLs**: Admin can configure webhook endpoints
2. **Triggers on Events**: Sends HTTP POST on flag create/update/delete
3. **Event Filtering**: Subscribe to specific events, projects, or environments
4. **Retry Logic**: Retries failed deliveries with exponential backoff
5. **Signature Verification**: Signs payloads with HMAC for security
6. **Delivery Tracking**: Logs webhook attempts and responses

## Concrete Tasks

### Phase 1: Database Schema
- [ ] Create `webhooks` table:
  ```sql
  CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url TEXT NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    
    -- Event filtering
    events TEXT[] NOT NULL,      -- ['flag.created', 'flag.updated', 'flag.deleted']
    project_id UUID REFERENCES projects(id),  -- NULL = all projects
    environments TEXT[],          -- ['prod', 'staging'] or NULL = all
    
    -- Security
    secret TEXT NOT NULL,         -- HMAC signing key
    
    -- Retry configuration
    max_retries INTEGER NOT NULL DEFAULT 3,
    timeout_seconds INTEGER NOT NULL DEFAULT 10,
    
    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_triggered_at TIMESTAMPTZ
  );
  
  CREATE INDEX idx_webhooks_enabled ON webhooks(enabled) WHERE enabled = true;
  CREATE INDEX idx_webhooks_project ON webhooks(project_id);
  ```
- [ ] Create `webhook_deliveries` table (delivery log):
  ```sql
  CREATE TABLE webhook_deliveries (
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
  ```
- [ ] Create migration: `202501180001_create_webhooks.sql`

### Phase 2: Webhook Management API
- [ ] Create sqlc queries in `internal/db/queries/webhooks.sql`:
  ```sql
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
    url = $2, description = $3, enabled = $4, events = $5,
    project_id = $6, environments = $7, max_retries = $8,
    timeout_seconds = $9, updated_at = now()
  WHERE id = $1;
  
  -- name: DeleteWebhook :exec
  DELETE FROM webhooks WHERE id = $1;
  
  -- name: GetActiveWebhooks :many
  SELECT * FROM webhooks WHERE enabled = true;
  ```
- [ ] Create REST endpoints:
  ```
  GET    /v1/admin/webhooks           - List all webhooks
  POST   /v1/admin/webhooks           - Create webhook
  GET    /v1/admin/webhooks/:id       - Get webhook details
  PUT    /v1/admin/webhooks/:id       - Update webhook
  DELETE /v1/admin/webhooks/:id       - Delete webhook
  GET    /v1/admin/webhooks/:id/deliveries  - List delivery attempts
  POST   /v1/admin/webhooks/:id/test - Test webhook (manual trigger)
  ```

### Phase 3: Event Payload Format
- [ ] Define standard webhook payload:
  ```json
  {
    "event": "flag.updated",
    "timestamp": "2025-01-15T10:30:00Z",
    "project": "customer-a",
    "environment": "prod",
    "resource": {
      "type": "flag",
      "key": "feature_x"
    },
    "data": {
      "before": {
        "key": "feature_x",
        "enabled": true,
        "rollout": 50,
        "config": {"color": "red"}
      },
      "after": {
        "key": "feature_x",
        "enabled": false,
        "rollout": 50,
        "config": {"color": "red"}
      },
      "changes": {
        "enabled": {"before": true, "after": false}
      }
    },
    "metadata": {
      "apiKeyId": "key-123",
      "ipAddress": "192.168.1.100",
      "requestId": "req-456"
    }
  }
  ```
- [ ] Support different event types:
  - `flag.created`
  - `flag.updated`
  - `flag.deleted`
  - `project.created` (if Issue #11 implemented)
  - `project.deleted` (if Issue #11 implemented)
  - `apikey.created` (if Issue #2 implemented)
  - `apikey.revoked` (if Issue #2 implemented)

### Phase 4: Webhook Dispatcher
- [ ] Create `internal/webhook/dispatcher.go`:
  ```go
  type Dispatcher struct {
    repo   *repo.Repo
    client *http.Client
    queue  chan Event
  }
  
  type Event struct {
    Type        string
    Timestamp   time.Time
    Project     string
    Environment string
    Resource    Resource
    Data        EventData
    Metadata    Metadata
  }
  
  func (d *Dispatcher) Start() {
    go d.worker()
  }
  
  func (d *Dispatcher) Dispatch(event Event) {
    d.queue <- event
  }
  
  func (d *Dispatcher) worker() {
    for event := range d.queue {
      webhooks, _ := d.getMatchingWebhooks(event)
      for _, webhook := range webhooks {
        d.deliverWithRetry(webhook, event)
      }
    }
  }
  ```
- [ ] Implement matching logic:
  ```go
  func (d *Dispatcher) getMatchingWebhooks(event Event) ([]*Webhook, error) {
    // Filter by:
    // 1. Enabled = true
    // 2. Event type in webhook.Events
    // 3. Project matches (if webhook.ProjectID != nil)
    // 4. Environment matches (if webhook.Environments != nil)
  }
  ```

### Phase 5: Delivery with Retry
- [ ] Implement delivery function:
  ```go
  func (d *Dispatcher) deliverWithRetry(webhook *Webhook, event Event) {
    payload, _ := json.Marshal(event)
    signature := computeHMAC(payload, webhook.Secret)
    
    for attempt := 0; attempt <= webhook.MaxRetries; attempt++ {
      start := time.Now()
      
      req, _ := http.NewRequest("POST", webhook.URL, bytes.NewReader(payload))
      req.Header.Set("Content-Type", "application/json")
      req.Header.Set("X-Flagship-Signature", signature)
      req.Header.Set("X-Flagship-Event", event.Type)
      req.Header.Set("X-Flagship-Delivery", deliveryID)
      
      ctx, cancel := context.WithTimeout(context.Background(), 
        time.Duration(webhook.TimeoutSeconds)*time.Second)
      defer cancel()
      
      resp, err := d.client.Do(req.WithContext(ctx))
      duration := time.Since(start)
      
      // Log delivery
      success := (err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300)
      d.logDelivery(webhook.ID, event, resp, err, duration, attempt, success)
      
      if success {
        return  // Success, no retry needed
      }
      
      // Exponential backoff
      if attempt < webhook.MaxRetries {
        time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
      }
    }
  }
  ```
- [ ] Implement HMAC signature:
  ```go
  func computeHMAC(payload []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    return "sha256=" + hex.EncodeToString(mac.Sum(nil))
  }
  ```

### Phase 6: Integration Points
- [ ] Trigger webhooks on flag mutations:
  ```go
  func (s *Server) handleUpsertFlag(w http.ResponseWriter, r *http.Request) {
    // ... existing logic ...
    
    // Trigger webhook
    s.webhookDispatcher.Dispatch(webhook.Event{
      Type:        "flag.updated",  // or "flag.created"
      Timestamp:   time.Now(),
      Project:     projectID,
      Environment: env,
      Resource:    webhook.Resource{Type: "flag", Key: flagKey},
      Data:        webhook.EventData{Before: oldState, After: newState, Changes: diff},
      Metadata:    webhook.Metadata{APIKeyID: apiKeyID, IPAddress: ip, RequestID: reqID},
    })
  }
  ```
- [ ] Integrate with all mutation endpoints
- [ ] Make webhook dispatch non-blocking (async)

### Phase 7: Admin UI for Webhooks
- [ ] Add "Webhooks" section to admin UI
- [ ] List all webhooks with status (enabled/disabled)
- [ ] Form to create/edit webhooks:
  - URL input
  - Event checkboxes (flag.created, flag.updated, etc.)
  - Project filter (dropdown)
  - Environment filter (multi-select)
  - Secret input (auto-generated, can be regenerated)
- [ ] Show recent deliveries:
  - Success/failure status
  - Response code
  - Duration
  - Retry count
  - Error message (if failed)
- [ ] Test button to manually trigger webhook

### Phase 8: Testing & Documentation
- [ ] Unit tests for HMAC signature
- [ ] Unit tests for event matching logic
- [ ] Integration tests with mock webhook server
- [ ] Test retry logic (simulate failures)
- [ ] Document webhook payload format
- [ ] Document signature verification for receivers
- [ ] Add example webhook receivers:
  - Slack webhook
  - Discord webhook
  - Generic HTTP logger

## API Changes

### New Endpoints

**Webhook Management**
```
GET    /v1/admin/webhooks           - List webhooks
POST   /v1/admin/webhooks           - Create webhook
GET    /v1/admin/webhooks/:id       - Get webhook
PUT    /v1/admin/webhooks/:id       - Update webhook
DELETE /v1/admin/webhooks/:id       - Delete webhook
GET    /v1/admin/webhooks/:id/deliveries  - List deliveries (paginated)
POST   /v1/admin/webhooks/:id/test - Test webhook
```

### Request/Response Examples

**Create Webhook**
```bash
curl -X POST http://localhost:8080/v1/admin/webhooks \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://hooks.slack.com/services/xxx",
    "description": "Notify #ops channel on prod flag changes",
    "events": ["flag.updated", "flag.deleted"],
    "project_id": "customer-a",
    "environments": ["prod"],
    "max_retries": 3,
    "timeout_seconds": 10
  }'

# Response
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://hooks.slack.com/services/xxx",
  "description": "Notify #ops channel on prod flag changes",
  "enabled": true,
  "events": ["flag.updated", "flag.deleted"],
  "project_id": "customer-a",
  "environments": ["prod"],
  "secret": "whsec_randomgeneratedsecret123",  # For HMAC verification
  "max_retries": 3,
  "timeout_seconds": 10,
  "created_at": "2025-01-15T10:30:00Z"
}
```

**List Webhook Deliveries**
```bash
curl "http://localhost:8080/v1/admin/webhooks/550e8400-e29b-41d4-a716-446655440000/deliveries?page=1&limit=20" \
  -H "Authorization: Bearer $ADMIN_API_KEY"

# Response
{
  "deliveries": [
    {
      "id": "delivery-123",
      "event_type": "flag.updated",
      "timestamp": "2025-01-15T10:30:05Z",
      "status_code": 200,
      "duration_ms": 245,
      "success": true,
      "retry_count": 0
    },
    {
      "id": "delivery-124",
      "event_type": "flag.deleted",
      "timestamp": "2025-01-15T11:00:00Z",
      "status_code": 500,
      "duration_ms": 10000,
      "success": false,
      "retry_count": 3,
      "error_message": "context deadline exceeded"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 47
  }
}
```

### Webhook Payload (Sent to External URL)
```json
{
  "event": "flag.updated",
  "timestamp": "2025-01-15T10:30:00Z",
  "project": "customer-a",
  "environment": "prod",
  "resource": {
    "type": "flag",
    "key": "feature_x"
  },
  "data": {
    "before": {
      "key": "feature_x",
      "enabled": true,
      "rollout": 50
    },
    "after": {
      "key": "feature_x",
      "enabled": false,
      "rollout": 50
    },
    "changes": {
      "enabled": {"before": true, "after": false}
    }
  },
  "metadata": {
    "apiKeyId": "key-123",
    "ipAddress": "192.168.1.100",
    "requestId": "req-456"
  }
}
```

### HTTP Headers (Sent with Webhook)
```
Content-Type: application/json
X-Flagship-Signature: sha256=abc123...
X-Flagship-Event: flag.updated
X-Flagship-Delivery: delivery-550e8400...
```

## Acceptance Criteria

### Webhook Management
- [ ] Webhooks can be created, listed, updated, deleted via API
- [ ] Webhook secrets are auto-generated (cryptographically secure)
- [ ] Webhooks can be enabled/disabled
- [ ] Event filtering works (specific events, projects, environments)

### Webhook Delivery
- [ ] Webhooks trigger on flag create/update/delete
- [ ] Payload includes before/after state and changes
- [ ] HMAC signature is included in headers
- [ ] Delivery is retried on failure (up to max_retries)
- [ ] Exponential backoff between retries
- [ ] Timeout is enforced (configurable per webhook)
- [ ] Delivery is non-blocking (doesn't slow down API)

### Delivery Tracking
- [ ] All delivery attempts are logged to database
- [ ] Success/failure status is recorded
- [ ] Response code and body are logged
- [ ] Duration is tracked
- [ ] Delivery logs can be queried via API

### Security
- [ ] HMAC signature allows receivers to verify authenticity
- [ ] Webhook URLs support HTTPS only (optional enforcement)
- [ ] Secrets are stored securely (hashed or encrypted)
- [ ] Failed deliveries don't expose sensitive data in logs

### Admin UI
- [ ] Webhooks can be managed from UI
- [ ] Recent deliveries are visible
- [ ] Test button works (manual trigger)
- [ ] Success/failure indicators are clear

## Notes / Risks / Edge Cases

### Risks
- **Security**: Webhook secrets could leak
  - Mitigation: Generate strong secrets, support rotation
- **DoS**: Malicious webhook URL could hang or crash server
  - Mitigation: Strict timeout, connection pool limits
- **Retry Storm**: Many failed webhooks could overwhelm system
  - Mitigation: Global rate limit on webhook dispatch, circuit breaker
- **Data Privacy**: Payloads might contain sensitive data
  - Mitigation: Allow payload filtering, redact sensitive fields

### Edge Cases
- Webhook URL is unreachable (DNS failure, network error)
- Webhook URL times out (slow receiver)
- Webhook URL returns 500 (receiver error)
- Webhook URL returns 429 (rate limited) → should we retry?
- Webhook is deleted while delivery in progress
- Same event triggers multiple webhooks (should all fire)
- Very large payloads (>1MB) → should we limit size?
- Webhook endpoint requires authentication → support custom headers?

### Signature Verification (Receiver Side)

Example verification code for webhook receivers:

**Node.js**
```javascript
const crypto = require('crypto');

function verifySignature(payload, signature, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(payload);
  const computed = 'sha256=' + hmac.digest('hex');
  return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(computed));
}

// Express middleware
app.post('/webhook', (req, res) => {
  const signature = req.headers['x-flagship-signature'];
  const payload = JSON.stringify(req.body);
  
  if (!verifySignature(payload, signature, process.env.WEBHOOK_SECRET)) {
    return res.status(401).send('Invalid signature');
  }
  
  // Process webhook event
  console.log('Received event:', req.body.event);
  res.sendStatus(200);
});
```

**Python**
```python
import hmac
import hashlib

def verify_signature(payload, signature, secret):
    computed = 'sha256=' + hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, computed)
```

### Best Practices for Webhook Receivers
- Always verify signature before processing
- Return 2xx quickly (process async if slow)
- Implement idempotency (same event ID might arrive twice)
- Log all webhooks for debugging
- Handle missing/extra fields gracefully
- Set reasonable timeout on your side

### Future Enhancements
- Webhook templates (pre-configured for Slack, Discord, etc.)
- Custom HTTP headers per webhook
- Payload templates (customize what's sent)
- Webhook batching (send multiple events in one request)
- Circuit breaker (stop trying if webhook consistently fails)
- Webhook analytics dashboard
- Event replay (resend past events)
- Dead letter queue (permanently failed events)

## Implementation Hints

- Webhook dispatcher should run in separate goroutine(s)
- Use buffered channel for event queue (size: 1000)
- HTTP client should have connection pooling and timeout
- Consider using `net/http/httptrace` for detailed metrics
- Store secrets as plain text (they're needed for signing) but limit access
- Use database transactions to ensure webhook isn't deleted during delivery
- Example HMAC verification:
  ```go
  func verifySignature(payload []byte, signature string, secret string) bool {
    expected := computeHMAC(payload, secret)
    return hmac.Equal([]byte(signature), []byte(expected))
  }
  ```
- Consider webhook queue persistence (save to DB if server crashes)

## Labels

`feature`, `backend`, `integration`, `good-first-issue` (for HMAC tests)

## Estimated Effort

**3-4 days**
- Day 1: Database schema + webhook CRUD API
- Day 2: Dispatcher + delivery logic + retry mechanism
- Day 3: Integration with flag mutations + delivery tracking
- Day 4: Admin UI + testing + documentation
