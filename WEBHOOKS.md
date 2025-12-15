# Webhooks Documentation

## Overview

Webhooks allow you to receive real-time notifications when flags change in goflagship. When a flag is created, updated, or deleted, the system can send HTTP POST requests to your configured webhook endpoints.

## Features

- **Event Filtering**: Subscribe to specific event types (create, update, delete)
- **Environment Filtering**: Filter events by environment (prod, staging, dev, etc.)
- **Automatic Retries**: Failed deliveries are automatically retried with exponential backoff
- **Signature Verification**: All webhook payloads are signed with HMAC-SHA256 for security
- **Delivery Tracking**: View logs of all webhook delivery attempts

## Webhook Endpoints

### Create Webhook

```http
POST /v1/admin/webhooks
Authorization: Bearer {admin_api_key}
Content-Type: application/json

{
  "url": "https://your-domain.com/webhook",
  "description": "Notify on prod flag changes",
  "events": ["flag.created", "flag.updated", "flag.deleted"],
  "environments": ["prod"],
  "max_retries": 3,
  "timeout_seconds": 10
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://your-domain.com/webhook",
  "description": "Notify on prod flag changes",
  "enabled": true,
  "events": ["flag.created", "flag.updated", "flag.deleted"],
  "environments": ["prod"],
  "secret": "whsec_abc123...",
  "max_retries": 3,
  "timeout_seconds": 10,
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

### List Webhooks

```http
GET /v1/admin/webhooks
Authorization: Bearer {admin_api_key}
```

### Get Webhook

```http
GET /v1/admin/webhooks/{id}
Authorization: Bearer {admin_api_key}
```

### Update Webhook

```http
PUT /v1/admin/webhooks/{id}
Authorization: Bearer {admin_api_key}
Content-Type: application/json

{
  "url": "https://your-domain.com/webhook",
  "description": "Updated description",
  "enabled": true,
  "events": ["flag.updated"],
  "environments": ["prod", "staging"],
  "max_retries": 5,
  "timeout_seconds": 15
}
```

### Delete Webhook

```http
DELETE /v1/admin/webhooks/{id}
Authorization: Bearer {admin_api_key}
```

### List Webhook Deliveries

```http
GET /v1/admin/webhooks/{id}/deliveries?page=1&limit=20
Authorization: Bearer {admin_api_key}
```

**Response:**
```json
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
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 47
  }
}
```

### Test Webhook

Manually trigger a test webhook delivery:

```http
POST /v1/admin/webhooks/{id}/test
Authorization: Bearer {admin_api_key}
```

## Webhook Payload Format

When an event occurs, your webhook endpoint will receive a POST request with the following format:

### Headers

```
Content-Type: application/json
X-Flagship-Signature: sha256=abc123...
X-Flagship-Event: flag.updated
X-Flagship-Delivery: delivery-550e8400...
```

### Payload

```json
{
  "event": "flag.updated",
  "timestamp": "2025-01-15T10:30:00Z",
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
      "enabled": {
        "before": true,
        "after": false
      }
    }
  },
  "metadata": {
    "apiKeyId": "key-123",
    "ipAddress": "192.168.1.100",
    "requestId": "req-456"
  }
}
```

## Event Types

- `flag.created` - Triggered when a new flag is created
- `flag.updated` - Triggered when an existing flag is updated
- `flag.deleted` - Triggered when a flag is deleted

## Signature Verification

All webhook payloads are signed with HMAC-SHA256 using your webhook's secret key. You should verify the signature before processing the webhook to ensure it came from goflagship.

### Verification Examples

#### Node.js

```javascript
const crypto = require('crypto');

function verifySignature(payload, signature, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(payload);
  const computed = 'sha256=' + hmac.digest('hex');
  return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(computed));
}

// Express middleware
app.post('/webhook', express.raw({ type: 'application/json' }), (req, res) => {
  const signature = req.headers['x-flagship-signature'];
  const payload = req.body.toString(); // raw body as string
  
  if (!verifySignature(payload, signature, process.env.WEBHOOK_SECRET)) {
    return res.status(401).send('Invalid signature');
  }
  
  // Process webhook event
  const event = JSON.parse(payload);
  console.log('Received event:', event.event);
  res.sendStatus(200);
});
```

#### Python

```python
import hmac
import hashlib
from flask import Flask, request

app = Flask(__name__)

def verify_signature(payload, signature, secret):
    computed = 'sha256=' + hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, computed)

@app.route('/webhook', methods=['POST'])
def webhook():
    signature = request.headers.get('X-Flagship-Signature')
    payload = request.get_data(as_text=True)
    
    if not verify_signature(payload, signature, os.environ['WEBHOOK_SECRET']):
        return 'Invalid signature', 401
    
    # Process webhook event
    event = request.get_json()
    print(f"Received event: {event['event']}")
    return 'OK', 200
```

#### Go

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

func verifySignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	computed := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(computed))
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("X-Flagship-Signature")
	payload, _ := io.ReadAll(r.Body)
	
	if !verifySignature(payload, signature, os.Getenv("WEBHOOK_SECRET")) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	
	var event map[string]interface{}
	json.Unmarshal(payload, &event)
	
	// Process webhook event
	println("Received event:", event["event"])
	w.WriteHeader(http.StatusOK)
}
```

## Best Practices

### For Webhook Receivers

1. **Always verify signatures** before processing webhooks
2. **Return 2xx quickly** - Process webhooks asynchronously if needed
3. **Implement idempotency** - The same event might be delivered multiple times
4. **Log all webhooks** for debugging purposes
5. **Handle missing/extra fields gracefully** - The payload format may evolve
6. **Set reasonable timeouts** on your side (10-30 seconds)

### For Webhook Configuration

1. **Use HTTPS** for webhook URLs in production
2. **Filter events** - Only subscribe to events you need
3. **Filter environments** - Separate staging and production webhooks
4. **Monitor delivery logs** - Check for failed deliveries regularly
5. **Test webhooks** - Use the test endpoint to verify your implementation
6. **Rotate secrets** - Periodically update webhook secrets for security

## Retry Behavior

- Failed deliveries are automatically retried up to `max_retries` times
- Exponential backoff is used between retries (1s, 2s, 4s, etc.)
- A delivery is considered successful if the response code is 2xx
- All delivery attempts are logged and can be viewed via the API

## Rate Limits

- Webhook delivery is non-blocking and does not slow down the API
- There is an internal queue of 1000 events
- If the queue is full, events may be dropped (rare in practice)

## Common Integration Examples

### Slack Notification

```javascript
// Webhook receiver that forwards to Slack
app.post('/webhook', async (req, res) => {
  const event = req.body;
  
  if (event.event === 'flag.updated' && event.environment === 'prod') {
    const message = {
      text: `⚠️ Flag *${event.resource.key}* was updated in production`,
      blocks: [
        {
          type: "section",
          text: {
            type: "mrkdwn",
            text: `Flag *${event.resource.key}* was ${event.data.after.enabled ? 'enabled' : 'disabled'}`
          }
        }
      ]
    };
    
    await fetch(process.env.SLACK_WEBHOOK_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(message)
    });
  }
  
  res.sendStatus(200);
});
```

### Datadog Event

```python
@app.route('/webhook', methods=['POST'])
def webhook():
    event = request.get_json()
    
    if event['event'] == 'flag.updated':
        # Send to Datadog
        datadog_event = {
            'title': f"Flag {event['resource']['key']} updated",
            'text': f"Environment: {event['environment']}",
            'tags': [f"env:{event['environment']}", 'source:goflagship']
        }
        
        requests.post(
            'https://api.datadoghq.com/api/v1/events',
            headers={'DD-API-KEY': os.environ['DATADOG_API_KEY']},
            json=datadog_event
        )
    
    return 'OK', 200
```

## Troubleshooting

### Webhook not being triggered

1. Check that the webhook is enabled
2. Verify event type matches (flag.created, flag.updated, flag.deleted)
3. Check environment filter - ensure it matches the flag's environment
4. Look at delivery logs for error messages

### Signature verification failing

1. Use the raw request body (not parsed JSON)
2. Ensure you're using the correct secret from the webhook configuration
3. The signature format is `sha256=<hex>`, not just `<hex>`

### Deliveries failing

1. Check your webhook endpoint is accessible from the internet
2. Ensure your endpoint returns 2xx status codes
3. Check timeout settings - increase if your endpoint is slow
4. Review delivery logs for specific error messages

## Security Considerations

- **Keep secrets secure** - Store webhook secrets in environment variables
- **Verify signatures** - Always verify HMAC signatures before processing
- **Use HTTPS** - Use HTTPS URLs in production to prevent MITM attacks
- **Rate limit** - Implement rate limiting on your webhook receiver
- **Validate payloads** - Validate the webhook payload structure before processing
