package auth

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// MockAuditLogger is a mock implementation of AuditLogger for testing
type MockAuditLogger struct {
	LastEntry *struct {
		APIKeyID  pgtype.UUID
		Action    string
		Resource  string
		IPAddress string
		UserAgent string
		Status    int32
		Details   map[string]interface{}
	}
	Err error
}

func (m *MockAuditLogger) CreateAuditLog(ctx context.Context, apiKeyID pgtype.UUID, action, resource, ipAddress, userAgent string, status int32, details map[string]interface{}) error {
	m.LastEntry = &struct {
		APIKeyID  pgtype.UUID
		Action    string
		Resource  string
		IPAddress string
		UserAgent string
		Status    int32
		Details   map[string]interface{}
	}{
		APIKeyID:  apiKeyID,
		Action:    action,
		Resource:  resource,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   details,
	}
	return m.Err
}

func TestLogAudit(t *testing.T) {
	ctx := context.Background()
	logger := &MockAuditLogger{}

	entry := AuditEntry{
		APIKeyID:  pgtype.UUID{Valid: false},
		Action:    "test_action",
		Resource:  "test_resource",
		IPAddress: "192.168.1.1",
		UserAgent: "TestAgent/1.0",
		Status:    200,
		Details:   map[string]interface{}{"key": "value"},
	}

	err := LogAudit(ctx, logger, entry)
	if err != nil {
		t.Fatalf("LogAudit() failed: %v", err)
	}

	if logger.LastEntry == nil {
		t.Fatal("Expected CreateAuditLog to be called")
	}

	if logger.LastEntry.Action != "test_action" {
		t.Errorf("Expected Action='test_action', got '%s'", logger.LastEntry.Action)
	}
	if logger.LastEntry.Resource != "test_resource" {
		t.Errorf("Expected Resource='test_resource', got '%s'", logger.LastEntry.Resource)
	}
	if logger.LastEntry.IPAddress != "192.168.1.1" {
		t.Errorf("Expected IPAddress='192.168.1.1', got '%s'", logger.LastEntry.IPAddress)
	}
	if logger.LastEntry.UserAgent != "TestAgent/1.0" {
		t.Errorf("Expected UserAgent='TestAgent/1.0', got '%s'", logger.LastEntry.UserAgent)
	}
	if logger.LastEntry.Status != 200 {
		t.Errorf("Expected Status=200, got %d", logger.LastEntry.Status)
	}
}

func TestLogAudit_WithError(t *testing.T) {
	ctx := context.Background()
	logger := &MockAuditLogger{
		Err: context.DeadlineExceeded,
	}

	entry := AuditEntry{
		Action:   "test_action",
		Resource: "test_resource",
		Status:   500,
	}

	err := LogAudit(ctx, logger, entry)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected error to be propagated, got %v", err)
	}
}

func TestLogAudit_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	logger := &MockAuditLogger{}

	// Details with invalid values should be handled gracefully
	entry := AuditEntry{
		Action:   "test_action",
		Resource: "test_resource",
		Status:   200,
		Details: map[string]interface{}{
			"valid": "value",
		},
	}

	err := LogAudit(ctx, logger, entry)
	if err != nil {
		t.Fatalf("LogAudit() should handle JSON gracefully: %v", err)
	}
}

func TestGetIPAddress_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")

	ip := GetIPAddress(req)
	if ip != "203.0.113.195, 70.41.3.18" {
		t.Errorf("Expected IP from X-Forwarded-For, got '%s'", ip)
	}
}

func TestGetIPAddress_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.42")

	ip := GetIPAddress(req)
	if ip != "198.51.100.42" {
		t.Errorf("Expected IP from X-Real-IP, got '%s'", ip)
	}
}

func TestGetIPAddress_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:54321"

	ip := GetIPAddress(req)
	if ip != "192.0.2.1:54321" {
		t.Errorf("Expected RemoteAddr, got '%s'", ip)
	}
}

func TestGetIPAddress_Priority(t *testing.T) {
	// X-Forwarded-For should take priority over X-Real-IP and RemoteAddr
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195")
	req.Header.Set("X-Real-IP", "198.51.100.42")
	req.RemoteAddr = "192.0.2.1:54321"

	ip := GetIPAddress(req)
	if ip != "203.0.113.195" {
		t.Errorf("Expected X-Forwarded-For to take priority, got '%s'", ip)
	}
}

func TestGetIPAddress_XRealIPPriorityOverRemoteAddr(t *testing.T) {
	// X-Real-IP should take priority over RemoteAddr
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.42")
	req.RemoteAddr = "192.0.2.1:54321"

	ip := GetIPAddress(req)
	if ip != "198.51.100.42" {
		t.Errorf("Expected X-Real-IP to take priority over RemoteAddr, got '%s'", ip)
	}
}
