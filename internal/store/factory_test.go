package store

import (
	"context"
	"strings"
	"testing"
)

func TestNewStore_Memory(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(ctx, "memory", "")
	if err != nil {
		t.Fatalf("NewStore('memory') failed: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	// Verify it's a memory store by checking it can store and retrieve
	err = store.UpsertFlag(ctx, UpsertParams{
		Key:     "test",
		Enabled: true,
		Rollout: 100,
		Env:     "test",
	})
	if err != nil {
		t.Fatalf("UpsertFlag failed: %v", err)
	}

	flags, err := store.GetAllFlags(ctx, "test")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}
	if len(flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(flags))
	}

	store.Close()
}

func TestNewStore_UnsupportedType(t *testing.T) {
	ctx := context.Background()
	_, err := NewStore(ctx, "invalid-type", "")
	if err == nil {
		t.Fatal("Expected error for unsupported store type")
	}
	// The error message now includes additional context, so just check it contains the key parts
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported store type") || !strings.Contains(errMsg, "invalid-type") {
		t.Errorf("Expected error message to mention unsupported type and invalid-type, got '%s'", errMsg)
	}
}

func TestNewStore_PostgresWithInvalidDSN(t *testing.T) {
	ctx := context.Background()
	// Invalid DSN should fail during pool creation
	_, err := NewStore(ctx, "postgres", "invalid-dsn")
	if err == nil {
		t.Fatal("Expected error for invalid DSN")
	}
	// Just verify we got an error (the specific error depends on the DSN parser)
	t.Logf("Got expected error: %v", err)
}

func TestNewStore_EmptyDSNForMemory(t *testing.T) {
	ctx := context.Background()
	// Memory store doesn't need a DSN
	store, err := NewStore(ctx, "memory", "")
	if err != nil {
		t.Fatalf("NewStore('memory') with empty DSN failed: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	store.Close()
}

func TestNewStore_PostgresRequiresDSN(t *testing.T) {
	ctx := context.Background()
	// Postgres store requires a DSN
	_, err := NewStore(ctx, "postgres", "")
	if err == nil {
		t.Fatal("Expected error when creating postgres store with empty DSN")
	}
	if !strings.Contains(err.Error(), "DSN cannot be empty") {
		t.Errorf("Expected error about empty DSN, got: %v", err)
	}
}

func TestNewStore_CaseSensitivity(t *testing.T) {
	ctx := context.Background()
	
	// Store type should be case-sensitive (lowercase expected)
	_, err := NewStore(ctx, "Memory", "")
	if err == nil {
		t.Error("Expected error for 'Memory' (capital M)")
	}

	_, err = NewStore(ctx, "MEMORY", "")
	if err == nil {
		t.Error("Expected error for 'MEMORY' (all caps)")
	}

	// Correct case should work
	store, err := NewStore(ctx, "memory", "")
	if err != nil {
		t.Fatalf("NewStore('memory') should work: %v", err)
	}
	store.Close()
}
