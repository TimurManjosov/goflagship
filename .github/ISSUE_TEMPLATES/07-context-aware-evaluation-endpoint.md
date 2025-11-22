# Implement Context-Aware Evaluation Endpoint

## Problem / Motivation

Currently, flag evaluation happens entirely client-side in the SDK. This has limitations:

1. **Client Bundle Size**: Including all flags + evaluation logic increases bundle size
2. **Security**: Exposing all flag configs to client may leak sensitive information
3. **Server-Side Use Cases**: Backend services can't easily evaluate flags
4. **Consistent Evaluation**: Hard to ensure backend and frontend use same logic
5. **Compliance**: Some use cases require server-side evaluation (e.g., payment features)

Teams need a **server-side evaluation endpoint** that:
- Accepts user context
- Evaluates all flags (or specific flags) for that user
- Returns only the resolved values (enabled/disabled + config)
- Works for backend services, mobile apps, and browsers

## Proposed Solution

Add a new REST endpoint `/v1/flags/evaluate` that:

1. Accepts user context in request body
2. Evaluates expressions, rollouts, and targeting rules server-side
3. Returns resolved flag states (only what user should see)
4. Optionally filters to specific flag keys
5. Supports both GET (query params) and POST (JSON body) methods

## Concrete Tasks

### Phase 1: Evaluation Logic Abstraction
- [ ] Extract flag evaluation logic to shared package `internal/evaluation/`:
  ```go
  package evaluation
  
  type Context struct {
    UserID     string
    Attributes map[string]any
  }
  
  type Result struct {
    Key       string         `json:"key"`
    Enabled   bool           `json:"enabled"`
    Variant   string         `json:"variant,omitempty"`
    Config    map[string]any `json:"config,omitempty"`
  }
  
  // EvaluateFlag evaluates a single flag for given context
  func EvaluateFlag(flag snapshot.FlagView, ctx Context, salt string) Result
  
  // EvaluateAll evaluates all flags for given context
  func EvaluateAll(flags map[string]snapshot.FlagView, ctx Context, salt string) []Result
  ```
- [ ] Implement evaluation order:
  1. Check `enabled` field → if false, return disabled
  2. Evaluate `expression` (if present) → if false, return disabled
  3. Check `rollout` (if <100) → hash user ID to determine inclusion
  4. Determine `variant` (if configured)
  5. Return result with resolved config
- [ ] Add unit tests for evaluation logic

### Phase 2: Evaluation Endpoint
- [ ] Add POST `/v1/flags/evaluate` handler:
  ```go
  func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request)
  ```
- [ ] Request schema:
  ```json
  {
    "user": {
      "id": "user-123",
      "attributes": {
        "plan": "premium",
        "country": "US",
        "email": "user@example.com"
      }
    },
    "keys": ["feature_x", "banner"]  // optional, evaluate only these
  }
  ```
- [ ] Response schema:
  ```json
  {
    "flags": [
      {
        "key": "feature_x",
        "enabled": true,
        "variant": "treatment",
        "config": {"color": "blue"}
      },
      {
        "key": "banner",
        "enabled": false
      }
    ],
    "etag": "W/\"abc123...\"",
    "evaluatedAt": "2025-01-15T10:30:00Z"
  }
  ```
- [ ] Validate request:
  - User ID is required
  - Keys array (if present) contains valid flag keys
- [ ] Return 400 for invalid requests

### Phase 3: Query Parameter Support
- [ ] Add GET `/v1/flags/evaluate` variant:
  ```
  GET /v1/flags/evaluate?userId=user-123&plan=premium&country=US&keys=feature_x,banner
  ```
- [ ] Parse user context from query params:
  - `userId` (required)
  - Other params → attributes map
  - `keys` → comma-separated list
- [ ] Useful for simple server-side checks without JSON payload

### Phase 4: Optimization & Caching
- [ ] Add ETag support:
  - Include snapshot ETag in response
  - Accept If-None-Match header
  - Return 304 if flags unchanged AND user context unchanged
  - Challenge: context changes per user (can't use global ETag)
- [ ] Consider response caching:
  - Cache evaluation results per user+snapshot
  - TTL: 60 seconds
  - Invalidate on snapshot update
- [ ] Add telemetry:
  - Metric: `evaluation_requests_total`
  - Metric: `evaluation_duration_ms`
  - Metric: `evaluation_cache_hits`

### Phase 5: SDK Integration
- [ ] Add SDK method to use evaluation endpoint:
  ```typescript
  const client = new FlagshipClient({
    baseUrl: 'http://localhost:8080',
    mode: 'server-evaluated',  // or 'client-evaluated'
    user: { id: 'user-123', attributes: {...} }
  });
  
  await client.init();  // Calls /v1/flags/evaluate instead of /snapshot
  ```
- [ ] SDK switches between:
  - **Client-evaluated mode**: Fetch snapshot, evaluate locally (current behavior)
  - **Server-evaluated mode**: Call evaluate endpoint, use results directly
- [ ] Benefits of server-evaluated mode:
  - Smaller initial payload (only resolved flags)
  - No need to include expression evaluator in bundle
  - Server controls evaluation logic

### Phase 6: Authentication & Rate Limiting
- [ ] Decide on authentication:
  - **Option A**: Public endpoint (no auth) → allows anyone to query
  - **Option B**: Require API key (readonly or admin) → secure but less convenient
  - **Option C**: Optional auth (public by default, can be locked down)
  - **Recommendation**: Option C (configurable via env var)
- [ ] Add rate limiting:
  - Per IP: 300 req/min (higher than admin endpoints)
  - Per API key (if authenticated): 1000 req/min
- [ ] Add configuration:
  ```bash
  EVALUATION_AUTH_REQUIRED=false  # Make public or require auth
  EVALUATION_RATE_LIMIT=300       # Requests per minute per IP
  ```

### Phase 7: Testing & Documentation
- [ ] Integration tests:
  - Simple flag (no expression, no rollout) → always enabled
  - Flag with expression → evaluates correctly
  - Flag with rollout → correct percentage split
  - Flag with variants → returns correct variant
  - Invalid user ID → error
  - Missing flag key → not in response
- [ ] Performance tests:
  - Evaluate 1000 flags for 1 user → <100ms
  - Concurrent requests → no race conditions
- [ ] Documentation:
  - API reference with examples
  - Use cases (mobile apps, backend services)
  - Client-evaluated vs server-evaluated trade-offs
  - Security considerations

## API Changes

### New Endpoint

**POST /v1/flags/evaluate**

Request:
```json
{
  "user": {
    "id": "user-123",
    "attributes": {
      "plan": "premium",
      "country": "US",
      "email": "user@example.com",
      "appVersion": "2.1.0"
    }
  },
  "keys": ["feature_x", "banner"]  // optional
}
```

Response: 200 OK
```json
{
  "flags": [
    {
      "key": "feature_x",
      "enabled": true,
      "variant": "treatment",
      "config": {"color": "blue", "cta": "Try Now"}
    },
    {
      "key": "banner",
      "enabled": false
    }
  ],
  "etag": "W/\"d3f5a1b2c3d4e5f6\"",
  "evaluatedAt": "2025-01-15T10:30:00Z"
}
```

**GET /v1/flags/evaluate (Alternative)**

```
GET /v1/flags/evaluate?userId=user-123&plan=premium&country=US&keys=feature_x,banner
```

Response: Same as POST

### Configuration

```bash
# .env
EVALUATION_AUTH_REQUIRED=false     # Require API key for /evaluate
EVALUATION_RATE_LIMIT=300          # Requests per minute per IP
EVALUATION_CACHE_TTL=60            # Cache evaluation results (seconds)
```

## Acceptance Criteria

### Endpoint Functionality
- [ ] POST `/v1/flags/evaluate` accepts user context and returns resolved flags
- [ ] GET `/v1/flags/evaluate` works with query parameters
- [ ] `keys` parameter filters to specific flags
- [ ] Evaluation logic matches SDK client-side evaluation (same results)
- [ ] Invalid user ID → 400 Bad Request
- [ ] Empty flags → returns empty array (not error)

### Evaluation Correctness
- [ ] Flag with `enabled=false` → always disabled
- [ ] Flag with expression → evaluates correctly
- [ ] Flag with rollout → deterministic based on user ID
- [ ] Flag with variants → returns correct variant and config
- [ ] Evaluation order: enabled → expression → rollout → variant

### Performance
- [ ] Evaluating 100 flags takes <50ms
- [ ] Evaluating 1000 flags takes <500ms
- [ ] Concurrent requests don't interfere (thread-safe)
- [ ] Optional caching reduces redundant evaluations

### SDK Integration
- [ ] SDK can use server-evaluated mode
- [ ] Results match client-evaluated mode
- [ ] SSE updates work in server-evaluated mode

### Documentation
- [ ] API endpoint documented with examples
- [ ] Use cases explained (backend services, mobile)
- [ ] Client-evaluated vs server-evaluated trade-offs documented
- [ ] Security implications documented

## Notes / Risks / Edge Cases

### Risks
- **Performance**: Evaluating many flags per request could be slow
  - Mitigation: Benchmark and optimize, add caching
- **Caching Complexity**: Per-user caching is harder than global caching
  - Mitigation: Simple TTL-based cache, invalidate on snapshot change
- **Security**: Exposing evaluation logic might reveal business rules
  - Mitigation: Document that returned configs are visible to client
- **Consistency**: Server and client evaluation must match
  - Mitigation: Share evaluation logic, extensive tests

### Edge Cases
- User context has non-standard attributes → should work (arbitrary keys)
- User ID is empty string → treat as error or default behavior?
- Requested key doesn't exist → omit from response (not an error)
- Flag expression references attribute not in context → treat as null
- Concurrent snapshot update during evaluation → use atomic snapshot
- Very large user context (>1MB) → reject with 413 Payload Too Large

### Client-Evaluated vs Server-Evaluated Trade-offs

**Client-Evaluated (Current)**
- ✅ Offline support (flags cached locally)
- ✅ Lower server load (evaluation on client)
- ✅ Faster after initial load (no network round-trip)
- ❌ Larger bundle size (all flags + evaluator)
- ❌ Exposes all flag configs to client
- ❌ Client can manipulate evaluation logic

**Server-Evaluated (New)**
- ✅ Smaller bundle size (only resolved flags)
- ✅ More secure (server controls logic)
- ✅ Consistent evaluation (server is source of truth)
- ✅ Works for backend services
- ❌ Requires network call per evaluation
- ❌ No offline support
- ❌ Higher server load

**Recommendation**: Offer both modes, let users choose based on needs.

### Future Enhancements
- Batch evaluation for multiple users (e.g., for emails/reports)
- Streaming evaluation (SSE-based, push updates)
- Evaluation with custom salt per request
- Evaluation with override flags (testing)
- Evaluation analytics (track which flags evaluated true/false)
- Server-side SDK wrappers (Go, Python, Java) that use this endpoint

## Implementation Hints

- Place evaluation logic in new `internal/evaluation/` package
- Current flag structure is in `internal/snapshot/snapshot.go` (FlagView)
- Expression evaluation will need targeting package (from Issue #6)
- Rollout logic will need rollout package (from Issue #5)
- Handler goes in `internal/api/server.go` or separate `evaluate.go` file
- Use snapshot.Load() to get current flags atomically
- Example evaluation flow:
  ```go
  func EvaluateFlag(flag FlagView, ctx Context, salt string) Result {
    if !flag.Enabled {
      return Result{Key: flag.Key, Enabled: false}
    }
    if flag.Expression != nil {
      match, _ := targeting.Evaluate(*flag.Expression, ctx.Attributes)
      if !match {
        return Result{Key: flag.Key, Enabled: false}
      }
    }
    if flag.Rollout < 100 {
      if !rollout.IsRolledOut(ctx.UserID, flag.Key, flag.Rollout, salt) {
        return Result{Key: flag.Key, Enabled: false}
      }
    }
    // ... variant logic
    return Result{Key: flag.Key, Enabled: true, Config: flag.Config}
  }
  ```

## Labels

`feature`, `backend`, `api`, `enhancement`

## Estimated Effort

**2 days**
- Day 1: Evaluation logic abstraction + endpoint implementation + tests
- Day 2: SDK integration + optimization + documentation

## Dependencies

- Issue #5 (Rollout Engine) - for rollout evaluation
- Issue #6 (Targeting Rules) - for expression evaluation
