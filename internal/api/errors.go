package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// ErrorCode represents machine-readable error codes
type ErrorCode string

const (
	// General error codes
	ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"
	ErrCodeBadRequest     ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrCodeRateLimited    ErrorCode = "RATE_LIMITED"
	ErrCodeRequestTooLarge ErrorCode = "REQUEST_TOO_LARGE"

	// Validation error codes
	ErrCodeValidation     ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidJSON    ErrorCode = "INVALID_JSON"
	ErrCodeInvalidKey     ErrorCode = "INVALID_KEY"
	ErrCodeMissingField   ErrorCode = "MISSING_FIELD"
	ErrCodeInvalidRollout ErrorCode = "INVALID_ROLLOUT"
	ErrCodeInvalidEnv     ErrorCode = "INVALID_ENV"
	ErrCodeInvalidConfig  ErrorCode = "INVALID_CONFIG"
	ErrCodeSchemaViolation ErrorCode = "SCHEMA_VIOLATION"
	ErrCodeInvalidExpression ErrorCode = "INVALID_EXPRESSION"
	ErrCodeInvalidVariants ErrorCode = "INVALID_VARIANTS"
)

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Error     string            `json:"error"`               // HTTP status text
	Message   string            `json:"message"`             // Human-readable description
	Code      ErrorCode         `json:"code"`                // Machine-readable error code
	Fields    map[string]string `json:"fields,omitempty"`    // Field-level errors
	RequestID string            `json:"request_id,omitempty"` // Request ID for debugging
}

// NewErrorResponse creates a new error response
func NewErrorResponse(statusCode int, code ErrorCode, message string) *ErrorResponse {
	return &ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    code,
	}
}

// WithFields adds field-level errors to the response
func (e *ErrorResponse) WithFields(fields map[string]string) *ErrorResponse {
	e.Fields = fields
	return e
}

// WithRequestID adds a request ID to the response
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
}

// writeErrorResponse writes a structured error response to the http response writer
func writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, errResp *ErrorResponse) {
	// Add request ID from chi middleware if available
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		errResp.RequestID = reqID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errResp)
}

// ValidationError creates a validation error response with field-level details
func ValidationError(w http.ResponseWriter, r *http.Request, message string, fields map[string]string) {
	errResp := NewErrorResponse(http.StatusBadRequest, ErrCodeValidation, message).
		WithFields(fields)
	writeErrorResponse(w, r, http.StatusBadRequest, errResp)
}

// BadRequestError creates a bad request error response
func BadRequestError(w http.ResponseWriter, r *http.Request, code ErrorCode, message string) {
	errResp := NewErrorResponse(http.StatusBadRequest, code, message)
	writeErrorResponse(w, r, http.StatusBadRequest, errResp)
}

// BadRequestErrorWithFields creates a bad request error with field-level details
func BadRequestErrorWithFields(w http.ResponseWriter, r *http.Request, code ErrorCode, message string, fields map[string]string) {
	errResp := NewErrorResponse(http.StatusBadRequest, code, message).
		WithFields(fields)
	writeErrorResponse(w, r, http.StatusBadRequest, errResp)
}

// UnauthorizedError creates an unauthorized error response
func UnauthorizedError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusUnauthorized, ErrCodeUnauthorized, message)
	writeErrorResponse(w, r, http.StatusUnauthorized, errResp)
}

// ForbiddenError creates a forbidden error response
func ForbiddenError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusForbidden, ErrCodeForbidden, message)
	writeErrorResponse(w, r, http.StatusForbidden, errResp)
}

// InternalError creates an internal server error response
func InternalError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusInternalServerError, ErrCodeInternal, message)
	writeErrorResponse(w, r, http.StatusInternalServerError, errResp)
}

// NotFoundError creates a not found error response
func NotFoundError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusNotFound, ErrCodeNotFound, message)
	writeErrorResponse(w, r, http.StatusNotFound, errResp)
}

// RequestTooLargeError creates a request entity too large error response
func RequestTooLargeError(w http.ResponseWriter, r *http.Request, message string) {
	errResp := NewErrorResponse(http.StatusRequestEntityTooLarge, ErrCodeRequestTooLarge, message)
	writeErrorResponse(w, r, http.StatusRequestEntityTooLarge, errResp)
}
