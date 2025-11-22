# Improve Validation and Error UX

## Problem / Motivation

The current system lacks comprehensive validation and user-friendly error handling:

1. **Weak Validation**: Flag config is free-form JSON with no schema validation
2. **Generic Errors**: HTTP errors like "invalid JSON" don't explain what's wrong
3. **No Frontend Feedback**: Admin UI doesn't properly display or handle errors
4. **Silent Failures**: Some validation errors are caught too late or not at all
5. **Inconsistent Error Format**: Error responses vary across endpoints

This leads to:
- Users submitting invalid flags that fail later
- Debugging difficulties when things go wrong
- Poor developer experience
- Increased support burden

## Proposed Solution

Implement three layers of validation and error handling:

1. **Backend JSON Schema Validation**: Optional schema enforcement for flag configs
2. **Structured Error Responses**: Consistent error format with field-level details
3. **Admin UI Error Handling**: Visual feedback for validation failures and network errors

## Concrete Tasks

### Phase 1: Backend Error Response Format
- [ ] Define standard error response structure:
  ```go
  type ErrorResponse struct {
      Error   string            `json:"error"`     // HTTP status text
      Message string            `json:"message"`   // Human-readable description
      Code    string            `json:"code"`      // Machine-readable error code
      Fields  map[string]string `json:"fields,omitempty"` // Field-level errors
  }
  ```
- [ ] Create error helper functions in `internal/api/errors.go`:
  - `ValidationError(field, message string)`
  - `BadRequestError(message string)`
  - `UnauthorizedError(message string)`
  - `InternalError(message string)`
- [ ] Update all handlers to use new error helpers
- [ ] Add error codes enum (e.g., `INVALID_KEY`, `MISSING_FIELD`, `SCHEMA_VIOLATION`)

### Phase 2: Request Validation Layer
- [ ] Create `internal/validation/validator.go` package
- [ ] Implement flag validation rules:
  - Key: non-empty, alphanumeric + underscore + hyphen, max 64 chars
  - Env: non-empty, max 32 chars
  - Rollout: 0-100 range
  - Description: max 500 chars
  - Config: valid JSON, max size (e.g., 100KB)
- [ ] Add validation before database operations
- [ ] Return field-level errors for multiple validation failures
- [ ] Add unit tests for all validation rules

### Phase 3: Optional JSON Schema Validation
- [ ] Add optional flag schema storage:
  - Add `schema` JSONB column to `flags` table (nullable)
  - Schema follows JSON Schema spec (draft-07)
- [ ] Integrate JSON schema validation library (e.g., `github.com/xeipuuv/gojsonschema`)
- [ ] Add endpoint `POST /v1/admin/flags/:key/schema` to set/update schema
- [ ] Validate flag config against schema on upsert (if schema exists)
- [ ] Add flag to enable/disable schema validation globally:
  ```bash
  FLAG_SCHEMA_VALIDATION_ENABLED=true
  ```
- [ ] Return detailed schema validation errors

### Phase 4: Admin UI Error Handling
- [ ] Update `sdk/admin.html` to display errors prominently:
  - Add error banner component with red styling
  - Show field-level errors inline (e.g., under form inputs)
  - Display network errors (connection refused, timeout)
- [ ] Improve form validation:
  - Client-side validation before submit (key format, rollout range)
  - Disable submit button during API calls
  - Show loading spinner
- [ ] Add toast notifications for success/error
- [ ] Handle edge cases:
  - 401/403 → show "Authentication failed" + redirect to login
  - 429 → show "Rate limited, retry in X seconds"
  - 500 → show "Server error" + suggest retry
  - Network offline → show "Connection lost" + retry button

### Phase 5: Error Logging and Debugging
- [ ] Log validation errors with context (user IP, key, timestamp)
- [ ] Add request ID to error responses for debugging
- [ ] Include request ID in logs for tracing
- [ ] Consider structured logging (JSON format) for production
- [ ] Add debug mode for verbose error messages:
  ```bash
  DEBUG_MODE=true  # Include stack traces in errors
  ```

### Phase 6: Documentation
- [ ] Document error codes and meanings
- [ ] Add examples of common validation errors
- [ ] Document JSON schema usage with examples
- [ ] Add troubleshooting guide for common errors
- [ ] Update API reference with error response format

## API Changes

### Error Response Format
All error responses will follow this structure:

```json
{
  "error": "Bad Request",
  "message": "Validation failed for one or more fields",
  "code": "VALIDATION_ERROR",
  "fields": {
    "key": "Key must be alphanumeric with underscores/hyphens only",
    "rollout": "Rollout must be between 0 and 100"
  }
}
```

### New Endpoints

**Set Flag Schema**
```
POST /v1/admin/flags/:key/schema
Content-Type: application/json
Authorization: Bearer <admin-key>

{
  "type": "object",
  "properties": {
    "message": { "type": "string", "maxLength": 200 },
    "color": { "type": "string", "enum": ["red", "blue", "green"] },
    "count": { "type": "integer", "minimum": 0 }
  },
  "required": ["message"],
  "additionalProperties": false
}
```

**Get Flag Schema**
```
GET /v1/admin/flags/:key/schema
Authorization: Bearer <admin-key>
```

### Example Error Scenarios

**Invalid Key Format**
```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer admin-123" \
  -d '{"key":"banner@message","enabled":true}'

# Response: 400 Bad Request
{
  "error": "Bad Request",
  "message": "Invalid flag key format",
  "code": "INVALID_KEY",
  "fields": {
    "key": "Key must contain only alphanumeric characters, underscores, and hyphens"
  }
}
```

**Schema Validation Failure**
```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer admin-123" \
  -d '{"key":"banner","enabled":true,"config":{"count":-5}}'

# Response: 400 Bad Request
{
  "error": "Bad Request",
  "message": "Config does not match schema",
  "code": "SCHEMA_VIOLATION",
  "fields": {
    "config.count": "Must be >= 0",
    "config.message": "Required field is missing"
  }
}
```

## Acceptance Criteria

### Backend
- [ ] All API endpoints return consistent error format
- [ ] Field-level validation errors are returned for all fields
- [ ] Error codes are machine-readable and documented
- [ ] JSON schema validation works for flags with schemas defined
- [ ] Schema validation can be enabled/disabled globally
- [ ] Invalid requests are rejected with 400, not 500
- [ ] Request IDs are included in error responses and logs
- [ ] All errors are logged with appropriate severity level

### Admin UI
- [ ] Validation errors are displayed prominently with red styling
- [ ] Field-level errors appear inline near form inputs
- [ ] Network errors show user-friendly messages
- [ ] Success actions show green confirmation toast
- [ ] Rate limit errors show countdown timer
- [ ] Form validation happens before submission
- [ ] Loading states prevent double-submission
- [ ] Error banner is dismissible

### Documentation
- [ ] Error code reference table exists
- [ ] JSON schema examples are provided
- [ ] Common error scenarios are documented
- [ ] Troubleshooting guide covers error debugging

## Notes / Risks / Edge Cases

### Risks
- **Breaking Changes**: New error format might break existing clients
  - Mitigation: Version the API (v1 keeps old format, v2 uses new format) OR use content negotiation
- **Performance**: JSON schema validation adds overhead
  - Mitigation: Make it optional, cache compiled schemas
- **Schema Complexity**: Users might create overly strict schemas
  - Mitigation: Document best practices, provide examples

### Edge Cases
- What if schema is invalid JSON Schema itself? Validate schema on upload
- Concurrent schema updates could cause race conditions
- Large schemas (>1MB) should be rejected
- Nested config validation (deep object structures)
- Array validation in configs
- Pattern matching in schemas (regex complexity)

### Validation Rules to Consider
- Key uniqueness across environments (current behavior?)
- Reserved key names (e.g., can't use internal prefixes)
- Config depth limits (prevent deeply nested objects)
- Expression syntax validation (if using DSL in future)
- URL validation in configs (if config contains URLs)

### UI/UX Considerations
- Don't overwhelm user with too many errors at once
- Prioritize errors (show critical ones first)
- Use progressive disclosure for nested errors
- Consider inline help text for each field
- Add "Learn more" links in error messages

### Future Enhancements
- OpenAPI/Swagger spec generation from validation rules
- GraphQL support with built-in validation
- Config preview/dry-run mode
- Validation webhooks (validate against external service)
- Custom validation rules per environment

## Implementation Hints

- Current error handling is in `internal/api/server.go`'s `writeError` function (lines 308-313)
- Request body parsing is in handler functions (e.g., `handleUpsertFlag`)
- Admin UI form handling is in `sdk/admin.html` (search for `fetch` calls)
- Consider using `github.com/go-playground/validator/v10` for struct validation
- JSON Schema library: `github.com/xeipuuv/gojsonschema` or `github.com/santhosh-tekuri/jsonschema`
- Error codes could be constants in `internal/api/errors.go`
- UI error components could use CSS animations for smooth appearance

## Labels

`feature`, `backend`, `frontend`, `ui`, `dx` (developer experience), `good-first-issue` (for validation unit tests)

## Estimated Effort

**2-3 days**
- Day 1: Backend error format + validation layer + unit tests
- Day 2: JSON schema integration + tests
- Day 3: Admin UI error handling + documentation
