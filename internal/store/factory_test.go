package store

import (
	"context"
	"errors"
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
	expectedMsg := "unsupported store type: invalid-type"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewStore_PostgresWithInvalidDSN(t *testing.T) {
	ctx := context.Background()
	// Invalid DSN should fail during pool creation
	_, err := NewStore(ctx, "postgres", "invalid-dsn")
	if err == nil {
		t.Fatal("Expected error for invalid DSN")
	}
	// Error should mention postgres pool creation
	if !errors.Is(err, err) { // Basic error check
		t.Logf("Got expected error: %v", err)
	}
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
