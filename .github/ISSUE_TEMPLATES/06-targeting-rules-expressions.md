# Implement Targeting Rules with Expression DSL

## Problem / Motivation

Currently, flags are either enabled or disabled for all users (or via rollout percentage). There's no way to:

1. **Target Specific Users**: Enable flag only for premium users, beta testers, or specific IDs
2. **Geographic Targeting**: Enable features based on country, region, or timezone
3. **Attribute-Based Rules**: Enable based on user plan, device type, app version, etc.
4. **Complex Logic**: Combine rules with AND/OR/NOT (e.g., "premium users in US OR beta testers")
5. **Dynamic Evaluation**: Change targeting without code deployment

Real-world examples:
- Show new checkout only to users in ["US", "CA"] with plan="premium"
- Enable mobile feature only if appVersion >= "2.0.0"
- Show beta features to users in betaTesterIds list OR with email ending in "@company.com"

## Proposed Solution

Implement a simple expression DSL for targeting rules:

1. **Expression Field**: Store rules as strings in database (already exists in schema)
2. **DSL Syntax**: JSON-based or custom syntax (e.g., CEL, JSON Logic)
3. **Evaluator**: Backend + SDK can evaluate expressions against user context
4. **Context Schema**: Define standard user attributes (id, email, country, plan, etc.)

**Approach**: Use existing well-tested expression languages rather than building from scratch. Recommendations:
- **JSON Logic** (simple, JSON-based, multi-language support)
- **CEL** (Common Expression Language by Google, powerful, standardized)

## Concrete Tasks

### Phase 1: Choose Expression Language
- [ ] Research options:
  - JSON Logic (jsonlogic.com) - simple, JSON-based
  - CEL (Common Expression Language) - Google standard
  - Custom DSL (not recommended, too much work)
- [ ] Evaluate based on:
  - Simplicity for non-technical users
  - TypeScript/Go library availability
  - Security (no code execution)
  - Performance
- [ ] **Decision**: Document chosen approach in issue comment
- [ ] Suggested: **JSON Logic** for simplicity

### Phase 2: Backend Expression Evaluator
- [ ] Add Go library (e.g., `github.com/diegoholiveira/jsonlogic` for JSON Logic)
- [ ] Create `internal/targeting/evaluator.go`:
  ```go
  type UserContext map[string]any
  
  // Evaluate returns true if user matches expression
  func Evaluate(expression string, context UserContext) (bool, error)
  ```
- [ ] Add expression validation on flag upsert:
  - Parse expression to ensure it's valid
  - Return 400 if malformed
- [ ] Add unit tests:
  - Simple rules: `{"==": [{"var": "plan"}, "premium"]}`
  - Complex rules: AND/OR/NOT combinations
  - Edge cases: missing variables, type mismatches

### Phase 3: SDK Expression Evaluator
- [ ] Add TypeScript library (e.g., `json-logic-js` npm package)
- [ ] Update SDK to evaluate expressions:
  ```typescript
  const client = new FlagshipClient({
    baseUrl: 'http://localhost:8080',
    user: {
      id: 'user-123',
      attributes: {
        plan: 'premium',
        country: 'US',
        email: 'user@example.com'
      }
    }
  });
  ```
- [ ] Update `isEnabled()` to check expression:
  ```typescript
  isEnabled(key: string): boolean {
    const flag = this.flags[key];
    if (!flag) return false;
    if (!flag.enabled) return false;
    
    // Check expression if present
    if (flag.expression) {
      const matches = this.evaluateExpression(flag.expression);
      if (!matches) return false;
    }
    
    // Check rollout if present
    if (flag.rollout < 100) {
      return this.checkRollout(key, flag.rollout);
    }
    
    return true;
  }
  ```
- [ ] Add SDK tests for expression evaluation

### Phase 4: Define User Context Schema
- [ ] Document standard attributes:
  ```typescript
  interface UserContext {
    id: string;                    // Required
    email?: string;
    plan?: string;                 // e.g., 'free', 'premium', 'enterprise'
    country?: string;              // ISO 3166-1 alpha-2 (e.g., 'US')
    region?: string;
    timezone?: string;             // e.g., 'America/New_York'
    locale?: string;               // e.g., 'en-US'
    deviceType?: string;           // e.g., 'mobile', 'desktop', 'tablet'
    appVersion?: string;           // e.g., '1.2.3'
    customAttributes?: Record<string, any>;
  }
  ```
- [ ] Add validation for context attributes (type checking)
- [ ] Document common expression patterns

### Phase 5: Admin UI Expression Builder
- [ ] Add expression field to flag create/edit form in `admin.html`
- [ ] Add visual rule builder (optional, can start with raw JSON):
  - Dropdown for operators (==, !=, in, <, >, and, or)
  - Autocomplete for attribute names
  - Input for values
- [ ] Add expression preview/testing:
  - Input test user context
  - Show whether expression evaluates to true/false
- [ ] Add example expressions in placeholder/help text

### Phase 6: Documentation & Examples
- [ ] Create expression syntax guide:
  - Basic comparisons
  - IN operator (list membership)
  - AND/OR/NOT logic
  - Nested rules
- [ ] Add common examples:
  ```json
  // Premium users only
  {"==": [{"var": "plan"}, "premium"]}
  
  // US or Canada
  {"in": [{"var": "country"}, ["US", "CA"]]}
  
  // Premium users in US
  {"and": [
    {"==": [{"var": "plan"}, "premium"]},
    {"==": [{"var": "country"}, "US"]}
  ]}
  
  // Beta testers OR internal employees
  {"or": [
    {"in": [{"var": "id"}, ["user-1", "user-2", "user-3"]]},
    {"match": [{"var": "email"}, "@company\\.com$"]}
  ]}
  
  // Version greater than or equal
  {">=": [{"var": "appVersion"}, "2.0.0"]}
  ```
- [ ] Add troubleshooting guide
- [ ] Document performance characteristics

### Phase 7: Security & Performance
- [ ] Add expression complexity limits:
  - Max expression size (e.g., 10KB)
  - Max evaluation depth (prevent deeply nested rules)
  - Timeout for evaluation (e.g., 10ms)
- [ ] Add rate limiting for expression evaluation (if server-side)
- [ ] Consider caching evaluated results (if context doesn't change)
- [ ] Security audit:
  - No code execution (JSON Logic doesn't allow this)
  - No access to sensitive data
  - Input sanitization

## API Changes

### Flag Schema Update
The `expression` field already exists but is now functional:

```json
{
  "key": "premium_feature",
  "enabled": true,
  "expression": "{\"==\": [{\"var\": \"plan\"}, \"premium\"]}",
  "rollout": 100,
  "config": {},
  "env": "prod"
}
```

### SDK Constructor Update
```typescript
const client = new FlagshipClient({
  baseUrl: 'http://localhost:8080',
  user: {
    id: 'user-123',
    attributes: {
      plan: 'premium',
      country: 'US',
      email: 'user@example.com',
      appVersion: '2.1.0'
    }
  }
});

// Update user context dynamically
client.updateUser({
  id: 'user-123',
  attributes: { plan: 'enterprise' }
});
```

### No New Endpoints
Expressions are evaluated client-side (in SDK) or server-side (in future evaluation endpoint).

## Acceptance Criteria

### Expression Evaluation
- [ ] Backend can evaluate JSON Logic expressions correctly
- [ ] SDK can evaluate JSON Logic expressions correctly
- [ ] Both backend and SDK produce same results for same input
- [ ] Invalid expressions are rejected on flag creation (400 error)
- [ ] Missing context attributes default to null/undefined (don't error)

### User Context
- [ ] SDK accepts user context in constructor
- [ ] User context can be updated dynamically
- [ ] Standard attributes are documented
- [ ] Custom attributes are supported

### Flag Behavior
- [ ] Flag with expression=null behaves as before (always enabled if enabled=true)
- [ ] Flag with expression evaluates expression first
- [ ] Expression false → flag disabled regardless of enabled field
- [ ] Expression true → proceed to rollout check
- [ ] Order: enabled → expression → rollout

### Performance
- [ ] Expression evaluation completes in <1ms (typical case)
- [ ] Complex expressions timeout at 10ms
- [ ] No memory leaks from repeated evaluations

### Documentation
- [ ] Expression syntax documented with examples
- [ ] User context schema documented
- [ ] Common patterns provided (premium users, geo-targeting, etc.)
- [ ] Migration guide for existing flags

## Notes / Risks / Edge Cases

### Risks
- **Complexity**: Expression DSL adds cognitive load for users
  - Mitigation: Provide visual rule builder, good examples, templates
- **Security**: Malicious expressions could cause DoS
  - Mitigation: Size limits, evaluation timeout, no code execution
- **Debugging**: Hard to understand why flag is disabled
  - Mitigation: Add debug mode that logs evaluation steps
- **Consistency**: Backend and SDK must evaluate identically
  - Mitigation: Use same library (JSON Logic has Go and JS implementations)

### Edge Cases
- Expression references attribute not in context → treat as null
- Expression has syntax error → reject on save, not evaluation
- Expression is very large (>10KB) → reject on save
- Expression is deeply nested (>10 levels) → reject or timeout
- User context is missing entirely → treat all variables as null
- Expression returns non-boolean (e.g., string) → coerce to boolean
- Concurrent expression evaluation (thread safety)

### Expression Language Trade-offs

**JSON Logic**
- ✅ Simple, JSON-based (easy to store/transmit)
- ✅ Libraries in many languages (Go, JS, Python, PHP)
- ✅ Well-documented, battle-tested
- ❌ Verbose syntax (lots of nested objects)
- ❌ Limited operator set (can add custom operators)

**CEL (Common Expression Language)**
- ✅ More readable syntax: `user.plan == "premium" && user.country == "US"`
- ✅ Google-backed, standardized
- ✅ Strong typing
- ❌ More complex to implement
- ❌ TypeScript library less mature

**Recommendation**: Start with JSON Logic for simplicity, consider CEL later if needed.

### Performance Considerations
- Expression evaluation happens on every flag check
- Cache results if user context doesn't change
- Consider batch evaluation (evaluate all flags at once)
- Profile with 100k flags to ensure scalability

### Future Enhancements
- Visual rule builder in admin UI (drag-and-drop)
- Expression templates library (common patterns)
- Expression testing/simulation in UI
- Server-side evaluation endpoint (for server-to-server flows)
- Expression versioning (rollback if expression breaks)
- A/B test with different expressions per variant
- Support for time-based rules (e.g., "enabled after 2025-06-01")

## Implementation Hints

- Expression field already exists in schema: `internal/db/migrations/202510230001_create_flags.sql` (line 10)
- Expression already in snapshot: `internal/snapshot/snapshot.go` (FlagView struct)
- Expression passed through in API: `internal/api/server.go` (upsertRequest struct, line 163)
- SDK structure: `sdk/flagshipClient.ts`
- Go library: `go get github.com/diegoholiveira/jsonlogic`
- JS library: `npm install json-logic-js`
- Example evaluation:
  ```go
  import "github.com/diegoholiveira/jsonlogic"
  
  rule := `{"==": [{"var": "plan"}, "premium"]}`
  data := `{"plan": "premium"}`
  result, _ := jsonlogic.Apply(rule, data)
  // result is true
  ```

## Labels

`feature`, `backend`, `sdk`, `good-first-issue` (for expression unit tests), `enhancement`

## Estimated Effort

**2-3 days**
- Day 1: Choose DSL + backend evaluator + unit tests
- Day 2: SDK evaluator + integration + tests
- Day 3: Admin UI updates + documentation + examples
