package snapshot

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestBuildFromFlags_Empty(t *testing.T) {
	flags := []store.Flag{}
	snap := BuildFromFlags(flags)

	if snap == nil {
		t.Fatal("BuildFromFlags returned nil")
	}
	if len(snap.Flags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(snap.Flags))
	}
	if snap.ETag == "" {
		t.Error("Expected non-empty ETag")
	}
}

func TestBuildFromFlags_MultipleFlags(t *testing.T) {
	now := time.Now().UTC()
	flags := []store.Flag{
		{
			Key:         "flag1",
			Description: "First flag",
			Enabled:     true,
			Rollout:     100,
			Env:         "prod",
			UpdatedAt:   now,
		},
		{
			Key:         "flag2",
			Description: "Second flag",
			Enabled:     false,
			Rollout:     50,
			Env:         "prod",
			UpdatedAt:   now,
		},
	}

	snap := BuildFromFlags(flags)

	if len(snap.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(snap.Flags))
	}

	flag1, ok := snap.Flags["flag1"]
	if !ok {
		t.Error("flag1 not found in snapshot")
	}
	if flag1.Key != "flag1" || !flag1.Enabled {
		t.Errorf("flag1 data incorrect: %+v", flag1)
	}

	flag2, ok := snap.Flags["flag2"]
	if !ok {
		t.Error("flag2 not found in snapshot")
	}
	if flag2.Key != "flag2" || flag2.Enabled {
		t.Errorf("flag2 data incorrect: %+v", flag2)
	}
}

func TestBuildFromFlags_ETags_Deterministic(t *testing.T) {
	flags := []store.Flag{
		{Key: "test", Enabled: true, Rollout: 50, Env: "prod", UpdatedAt: time.Now().UTC()},
	}

	snap1 := BuildFromFlags(flags)
	snap2 := BuildFromFlags(flags)

	if snap1.ETag != snap2.ETag {
		t.Errorf("Expected deterministic ETags, got %s and %s", snap1.ETag, snap2.ETag)
	}
}

func TestBuildFromFlags_ETags_Different(t *testing.T) {
	flags1 := []store.Flag{
		{Key: "flag1", Enabled: true, Rollout: 100, Env: "prod", UpdatedAt: time.Now().UTC()},
	}
	flags2 := []store.Flag{
		{Key: "flag2", Enabled: false, Rollout: 50, Env: "prod", UpdatedAt: time.Now().UTC()},
	}

	snap1 := BuildFromFlags(flags1)
	snap2 := BuildFromFlags(flags2)

	if snap1.ETag == snap2.ETag {
		t.Error("Expected different ETags for different flags")
	}
}

func TestBuildFromFlags_ConfigJSON(t *testing.T) {
	config := map[string]any{
		"text":  "Hello World",
		"color": "blue",
	}
	flags := []store.Flag{
		{
			Key:       "banner",
			Enabled:   true,
			Rollout:   100,
			Config:    config,
			Env:       "prod",
			UpdatedAt: time.Now().UTC(),
		},
	}

	snap := BuildFromFlags(flags)

	flag, ok := snap.Flags["banner"]
	if !ok {
		t.Fatal("banner flag not found")
	}

	if flag.Config["text"] != "Hello World" {
		t.Errorf("Expected config text 'Hello World', got %v", flag.Config["text"])
	}
	if flag.Config["color"] != "blue" {
		t.Errorf("Expected config color 'blue', got %v", flag.Config["color"])
	}
}

func TestLoadAndUpdate(t *testing.T) {
	// Load initial (should be empty)
	initial := Load()
	if initial == nil {
		t.Fatal("Load returned nil")
	}
	if len(initial.Flags) != 0 {
		t.Errorf("Expected empty initial snapshot, got %d flags", len(initial.Flags))
	}

	// Create and update
	flags := []store.Flag{
		{Key: "new_flag", Enabled: true, Rollout: 100, Env: "prod", UpdatedAt: time.Now().UTC()},
	}
	newSnap := BuildFromFlags(flags)
	Update(newSnap)

	// Load again
	loaded := Load()
	if len(loaded.Flags) != 1 {
		t.Errorf("Expected 1 flag after update, got %d", len(loaded.Flags))
	}
	if loaded.ETag != newSnap.ETag {
		t.Errorf("Expected ETag %s, got %s", newSnap.ETag, loaded.ETag)
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	// Subscribe
	updates, unsub := Subscribe()
	defer unsub()

	// Create a new snapshot and update
	flags := []store.Flag{
		{Key: "test", Enabled: true, Rollout: 100, Env: "prod", UpdatedAt: time.Now().UTC()},
	}
	snap := BuildFromFlags(flags)

	// Update in a goroutine to avoid blocking
	go func() {
		time.Sleep(10 * time.Millisecond)
		Update(snap)
	}()

	// Wait for update with timeout
	select {
	case etag := <-updates:
		if etag != snap.ETag {
			t.Errorf("Expected ETag %s, got %s", snap.ETag, etag)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for update")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	// Create multiple subscribers
	updates1, unsub1 := Subscribe()
	defer unsub1()
	updates2, unsub2 := Subscribe()
	defer unsub2()

	// Update snapshot
	flags := []store.Flag{
		{Key: "multi", Enabled: true, Rollout: 50, Env: "prod", UpdatedAt: time.Now().UTC()},
	}
	snap := BuildFromFlags(flags)
	Update(snap)

	// Both should receive the update
	timeout := time.After(1 * time.Second)
	received := 0

	for received < 2 {
		select {
		case etag := <-updates1:
			if etag == snap.ETag {
				received++
			}
		case etag := <-updates2:
			if etag == snap.ETag {
				received++
			}
		case <-timeout:
			t.Errorf("Timeout: only %d of 2 subscribers received update", received)
			return
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap := Load()
			if snap == nil {
				t.Error("Load returned nil")
			}
		}()
	}

	// Concurrent updates
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			flags := []store.Flag{
				{
					Key:       "concurrent_flag",
					Enabled:   true,
					Rollout:   int32(n % 100),
					Env:       "prod",
					UpdatedAt: time.Now().UTC(),
				},
			}
			snap := BuildFromFlags(flags)
			Update(snap)
		}(i)
	}

	wg.Wait()

	// Verify final state is valid
	final := Load()
	if final == nil {
		t.Error("Final Load returned nil")
	}
}

func TestETagFormat(t *testing.T) {
	flags := []store.Flag{
		{Key: "test", Enabled: true, Rollout: 100, Env: "prod", UpdatedAt: time.Now().UTC()},
	}
	snap := BuildFromFlags(flags)

	// ETag should start with W/" (weak ETag format)
	if len(snap.ETag) < 4 || snap.ETag[:3] != `W/"` {
		t.Errorf("Expected ETag to start with 'W/\"', got %s", snap.ETag)
	}

	// ETag should end with "
	if snap.ETag[len(snap.ETag)-1] != '"' {
		t.Errorf("Expected ETag to end with '\"', got %s", snap.ETag)
	}
}

func TestSnapshotMarshaling(t *testing.T) {
	flags := []store.Flag{
		{
			Key:         "json_test",
			Description: "Test JSON marshaling",
			Enabled:     true,
			Rollout:     75,
			Config:      map[string]any{"nested": map[string]any{"value": 42}},
			Env:         "prod",
			UpdatedAt:   time.Now().UTC(),
		},
	}
	snap := BuildFromFlags(flags)

	// Marshal to JSON
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	// Unmarshal back
	var unmarshaled Snapshot
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	if unmarshaled.ETag != snap.ETag {
		t.Errorf("ETag mismatch after unmarshal: %s != %s", unmarshaled.ETag, snap.ETag)
	}
	if len(unmarshaled.Flags) != len(snap.Flags) {
		t.Errorf("Flags count mismatch: %d != %d", len(unmarshaled.Flags), len(snap.Flags))
	}
}
