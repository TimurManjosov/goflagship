// Package api provides HTTP handlers and middleware for the flagship feature flag service.
// It includes structured error responses, authentication, rate limiting, and RESTful endpoints.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// ErrorCode represents machine-readable error codes for API responses.
// These codes allow clients to programmatically handle different error scenarios.
type ErrorCode string

const (
	// General error codes
	ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"       // Unexpected server error
	ErrCodeBadRequest     ErrorCode = "BAD_REQUEST"          // Malformed request
	ErrCodeUnauthorized   ErrorCode = "UNAUTHORIZED"         // Missing or invalid authentication
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"            // Insufficient permissions
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"            // Resource doesn't exist
	ErrCodeRateLimited    ErrorCode = "RATE_LIMITED"         // Too many requests
	ErrCodeRequestTooLarge ErrorCode = "REQUEST_TOO_LARGE"   // Request body too large

	// Validation error codes
	ErrCodeValidation        ErrorCode = "VALIDATION_ERROR"      // Generic validation failure
	ErrCodeInvalidJSON       ErrorCode = "INVALID_JSON"          // JSON parsing failed
	ErrCodeInvalidKey        ErrorCode = "INVALID_KEY"           // Flag key format invalid
	ErrCodeMissingField      ErrorCode = "MISSING_FIELD"         // Required field missing
	ErrCodeInvalidRollout    ErrorCode = "INVALID_ROLLOUT"       // Rollout % not in 0-100
	ErrCodeInvalidEnv        ErrorCode = "INVALID_ENV"           // Environment name invalid
	ErrCodeInvalidConfig     ErrorCode = "INVALID_CONFIG"        // Config JSON invalid
	ErrCodeSchemaViolation   ErrorCode = "SCHEMA_VIOLATION"      // Data doesn't match schema
	ErrCodeInvalidExpression ErrorCode = "INVALID_EXPRESSION"    // Targeting expression invalid
	ErrCodeInvalidVariants   ErrorCode = "INVALID_VARIANTS"      // A/B test variants invalid
)

// ErrorResponse represents a structured API error response.
// It provides both human-readable messages and machine-readable codes.
//
// Example JSON response:
//
//	{
//	  "error": "Bad Request",
//	  "message": "Flag key must be alphanumeric and between 3-64 characters",
//	  "code": "INVALID_KEY",
//	  "fields": {
//	    "key": "Must match pattern ^[a-zA-Z0-9_-]+$"
//	  },
//	  "request_id": "abc123"
//	}
type ErrorResponse struct {
	Error     string            `json:"error"`               // HTTP status text (e.g., "Bad Request")
	Message   string            `json:"message"`             // Human-readable error description
	Code      ErrorCode         `json:"code"`                // Machine-readable error code
	Fields    map[string]string `json:"fields,omitempty"`    // Field-level validation errors
	RequestID string            `json:"request_id,omitempty"` // Request ID for debugging/tracing
}

// NewErrorResponse creates a new error response with the given status code, error code, and message.
func NewErrorResponse(statusCode int, code ErrorCode, message string) *ErrorResponse {
	return &ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    code,
	}
}

// WithFields adds field-level validation errors to the response.
// Useful for showing which specific fields failed validation.
//
// Example:
//
//	errResp.WithFields(map[string]string{
//	  "email": "Must be a valid email address",
//	  "age": "Must be at least 18"
//	})
func (e *ErrorResponse) WithFields(fields map[string]string) *ErrorResponse {
	e.Fields = fields
	return e
}

// WithRequestID adds a request ID to the response for tracing and debugging.
// The request ID can be used to correlate client errors with server logs.
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
}

// writeErrorResponse writes a structured error response to the HTTP response writer.
// It automatically includes the request ID from chi middleware if available.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, errResp *ErrorResponse) {
	// Add request ID from chi middleware if available
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		errResp.RequestID = requestID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errResp)
}

// ValidationError creates a validation error response with field-level details.
//
// Usage:
//
//	ValidationError(w, r, "Invalid flag parameters", map[string]string{
//	  "key": "Must be alphanumeric",
//	  "rollout": "Must be between 0 and 100"
//	})
func ValidationError(w http.ResponseWriter, r *http.Request, message string, fields map[string]string) {
	errResp := NewErrorResponse(http.StatusBadRequest, ErrCodeValidation, message).
		WithFields(fields)
	writeErrorResponse(w, r, http.StatusBadRequest, errResp)
}

// BadRequestError creates a generic bad request error response.
//
// Usage:
//
//	BadRequestError(w, r, ErrCodeInvalidJSON, "Request body is not valid JSON")
func BadRequestError(w http.ResponseWriter, r *http.Request, code ErrorCode, message string) {
	errResp := NewErrorResponse(http.StatusBadRequest, code, message)
	writeErrorResponse(w, r, http.StatusBadRequest, errResp)
}

// BadRequestErrorWithFields creates a bad request error with field-level details.
//
// Usage:
//
//	BadRequestErrorWithFields(w, r, ErrCodeInvalidVariants, "Variant weights must sum to 100",
//	  map[string]string{"weights": "Current sum is 90"})
func BadRequestErrorWithFields(w http.ResponseWriter, r *http.Request, code ErrorCode, message string, fields map[string]string) {
	errResp := NewErrorResponse(http.StatusBadRequest, code, message).
		WithFields(fields)
	writeErrorResponse(w, r, http.StatusBadRequest, errResp)
}

// UnauthorizedError creates an unauthorized (401) error response.
//
// Usage:
//
//	UnauthorizedError(w, r, "Invalid or missing API key")
func UnauthorizedError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusUnauthorized, ErrCodeUnauthorized, message)
	writeErrorResponse(w, r, http.StatusUnauthorized, errResp)
}

// ForbiddenError creates a forbidden (403) error response.
//
// Usage:
//
//	ForbiddenError(w, r, "Insufficient permissions to delete flags")
func ForbiddenError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusForbidden, ErrCodeForbidden, message)
	writeErrorResponse(w, r, http.StatusForbidden, errResp)
}

// InternalError creates an internal server error (500) response.
//
// Usage:
//
//	InternalError(w, r, "Failed to connect to database")
func InternalError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusInternalServerError, ErrCodeInternal, message)
	writeErrorResponse(w, r, http.StatusInternalServerError, errResp)
}

// NotFoundError creates a not found (404) error response.
//
// Usage:
//
//	NotFoundError(w, r, "Flag 'feature_x' not found")
func NotFoundError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusNotFound, ErrCodeNotFound, message)
	writeErrorResponse(w, r, http.StatusNotFound, errResp)
}

// RequestTooLargeError creates a request entity too large (413) error response.
//
// Usage:
//
//	RequestTooLargeError(w, r, "Request body exceeds 1MB limit")
func RequestTooLargeError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusRequestEntityTooLarge, ErrCodeRequestTooLarge, message)
	writeErrorResponse(w, r, http.StatusRequestEntityTooLarge, errResp)
}
