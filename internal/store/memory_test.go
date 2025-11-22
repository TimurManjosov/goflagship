package store

import (
	"context"
	"testing"
)

func TestMemoryStore_UpsertAndGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Upsert a flag
	params := UpsertParams{
		Key:         "test-flag",
		Description: "Test flag description",
		Enabled:     true,
		Rollout:     50,
		Expression:  nil,
		Config:      map[string]any{"key": "value"},
		Env:         "prod",
	}

	err := store.UpsertFlag(ctx, params)
	if err != nil {
		t.Fatalf("UpsertFlag failed: %v", err)
	}

	// Get the flag by key
	flag, err := store.GetFlagByKey(ctx, "test-flag")
	if err != nil {
		t.Fatalf("GetFlagByKey failed: %v", err)
	}

	if flag.Key != "test-flag" {
		t.Errorf("Expected key 'test-flag', got '%s'", flag.Key)
	}
	if flag.Enabled != true {
		t.Errorf("Expected Enabled to be true, got false")
	}
	if flag.Rollout != 50 {
		t.Errorf("Expected Rollout to be 50, got %d", flag.Rollout)
	}
}

func TestMemoryStore_GetAllFlags(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Add multiple flags for different environments
	flags := []UpsertParams{
		{Key: "flag1", Description: "Flag 1", Enabled: true, Rollout: 100, Env: "prod"},
		{Key: "flag2", Description: "Flag 2", Enabled: false, Rollout: 0, Env: "prod"},
		{Key: "flag3", Description: "Flag 3", Enabled: true, Rollout: 50, Env: "dev"},
	}

	for _, f := range flags {
		if err := store.UpsertFlag(ctx, f); err != nil {
			t.Fatalf("UpsertFlag failed: %v", err)
		}
	}

	// Get all flags for prod environment
	prodFlags, err := store.GetAllFlags(ctx, "prod")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}

	if len(prodFlags) != 2 {
		t.Errorf("Expected 2 flags for prod, got %d", len(prodFlags))
	}

	// Get all flags for dev environment
	devFlags, err := store.GetAllFlags(ctx, "dev")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}

	if len(devFlags) != 1 {
		t.Errorf("Expected 1 flag for dev, got %d", len(devFlags))
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Insert a flag
	params := UpsertParams{
		Key:         "update-test",
		Description: "Original description",
		Enabled:     false,
		Rollout:     0,
		Env:         "prod",
	}

	err := store.UpsertFlag(ctx, params)
	if err != nil {
		t.Fatalf("Initial UpsertFlag failed: %v", err)
	}

	// Update the flag
	params.Description = "Updated description"
	params.Enabled = true
	params.Rollout = 100

	err = store.UpsertFlag(ctx, params)
	if err != nil {
		t.Fatalf("Update UpsertFlag failed: %v", err)
	}

	// Verify the update
	flag, err := store.GetFlagByKey(ctx, "update-test")
	if err != nil {
		t.Fatalf("GetFlagByKey failed: %v", err)
	}

	if flag.Description != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", flag.Description)
	}
	if flag.Enabled != true {
		t.Errorf("Expected Enabled to be true, got false")
	}
	if flag.Rollout != 100 {
		t.Errorf("Expected Rollout to be 100, got %d", flag.Rollout)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Insert a flag
	params := UpsertParams{
		Key:         "delete-test",
		Description: "To be deleted",
		Enabled:     true,
		Rollout:     50,
		Env:         "prod",
	}

	err := store.UpsertFlag(ctx, params)
	if err != nil {
		t.Fatalf("UpsertFlag failed: %v", err)
	}

	// Delete the flag
	err = store.DeleteFlag(ctx, "delete-test", "prod")
	if err != nil {
		t.Fatalf("DeleteFlag failed: %v", err)
	}

	// Verify deletion
	_, err = store.GetFlagByKey(ctx, "delete-test")
	if err == nil {
		t.Error("Expected error when getting deleted flag, got nil")
	}

	// Delete again (idempotent)
	err = store.DeleteFlag(ctx, "delete-test", "prod")
	if err != nil {
		t.Fatalf("Second DeleteFlag failed: %v", err)
	}
}

func TestMemoryStore_DeleteWrongEnv(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Insert a flag in prod
	params := UpsertParams{
		Key:         "env-test",
		Description: "Env test",
		Enabled:     true,
		Rollout:     50,
		Env:         "prod",
	}

	err := store.UpsertFlag(ctx, params)
	if err != nil {
		t.Fatalf("UpsertFlag failed: %v", err)
	}

	// Try to delete with wrong env
	err = store.DeleteFlag(ctx, "env-test", "dev")
	if err != nil {
		t.Fatalf("DeleteFlag failed: %v", err)
	}

	// Flag should still exist
	flag, err := store.GetFlagByKey(ctx, "env-test")
	if err != nil {
		t.Fatalf("GetFlagByKey failed: %v", err)
	}

	if flag.Env != "prod" {
		t.Errorf("Expected env 'prod', got '%s'", flag.Env)
	}
}

func TestMemoryStore_GetNonExistent(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, err := store.GetFlagByKey(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error when getting non-existent flag, got nil")
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()

	err := store.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}
