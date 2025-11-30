package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(http.StatusBadRequest, ErrCodeInvalidKey, "Invalid key format")

	if resp.Error != "Bad Request" {
		t.Errorf("Expected Error 'Bad Request', got '%s'", resp.Error)
	}
	if resp.Message != "Invalid key format" {
		t.Errorf("Expected Message 'Invalid key format', got '%s'", resp.Message)
	}
	if resp.Code != ErrCodeInvalidKey {
		t.Errorf("Expected Code ErrCodeInvalidKey, got '%s'", resp.Code)
	}
}

func TestErrorResponse_WithFields(t *testing.T) {
	fields := map[string]string{
		"key":     "Key is required",
		"rollout": "Rollout must be between 0 and 100",
	}

	resp := NewErrorResponse(http.StatusBadRequest, ErrCodeValidation, "Validation failed").
		WithFields(fields)

	if len(resp.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(resp.Fields))
	}
	if resp.Fields["key"] != "Key is required" {
		t.Errorf("Expected field 'key' to be 'Key is required', got '%s'", resp.Fields["key"])
	}
}

func TestErrorResponse_WithRequestID(t *testing.T) {
	resp := NewErrorResponse(http.StatusInternalServerError, ErrCodeInternal, "Internal error").
		WithRequestID("req-123")

	if resp.RequestID != "req-123" {
		t.Errorf("Expected RequestID 'req-123', got '%s'", resp.RequestID)
	}
}

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	fields := map[string]string{
		"key": "Key is required",
	}

	ValidationError(w, r, "Validation failed", fields)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeValidation {
		t.Errorf("Expected Code ErrCodeValidation, got '%s'", resp.Code)
	}
	if resp.Fields["key"] != "Key is required" {
		t.Errorf("Expected field 'key' error, got '%s'", resp.Fields["key"])
	}
}

func TestBadRequestError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeInvalidJSON {
		t.Errorf("Expected Code ErrCodeInvalidJSON, got '%s'", resp.Code)
	}
	if resp.Message != "Invalid JSON" {
		t.Errorf("Expected message 'Invalid JSON', got '%s'", resp.Message)
	}
}

func TestUnauthorizedError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	UnauthorizedError(w, r, "Missing authentication")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("Expected Code ErrCodeUnauthorized, got '%s'", resp.Code)
	}
}

func TestForbiddenError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	ForbiddenError(w, r, "Insufficient permissions")

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeForbidden {
		t.Errorf("Expected Code ErrCodeForbidden, got '%s'", resp.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	InternalError(w, r, "Database connection failed")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeInternal {
		t.Errorf("Expected Code ErrCodeInternal, got '%s'", resp.Code)
	}
}

func TestNotFoundError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/flags/unknown", nil)

	NotFoundError(w, r, "Flag not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeNotFound {
		t.Errorf("Expected Code ErrCodeNotFound, got '%s'", resp.Code)
	}
}

func TestRequestTooLargeError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	RequestTooLargeError(w, r, "Request body exceeds limit")

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeRequestTooLarge {
		t.Errorf("Expected Code ErrCodeRequestTooLarge, got '%s'", resp.Code)
	}
}

func TestErrorResponseContentType(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/flags", nil)

	BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON")

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}
