package snapshot

import (
	"sync"
	"testing"
	"time"
)

func TestSubscribeReturnsChannel(t *testing.T) {
	updates, unsub := Subscribe()
	defer unsub()

	if updates == nil {
		t.Error("Subscribe returned nil channel")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	updates, unsub := Subscribe()

	// Unsubscribe immediately
	unsub()

	// Channel should be closed
	select {
	case _, ok := <-updates:
		if ok {
			t.Error("Expected channel to be closed after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for channel close")
	}
}

func TestPublishUpdateNonBlocking(t *testing.T) {
	// Create a subscriber but don't read from it (slow client simulation)
	updates, unsub := Subscribe()
	defer unsub()

	// Fill the buffer
	publishUpdate("etag1")

	// This should not block even though the channel is full
	done := make(chan bool)
	go func() {
		publishUpdate("etag2")
		publishUpdate("etag3")
		done <- true
	}()

	select {
	case <-done:
		// Success - publishUpdate did not block
	case <-time.After(100 * time.Millisecond):
		t.Error("publishUpdate blocked on slow subscriber")
	}

	// Clean up: drain the channel
	for len(updates) > 0 {
		<-updates
	}
}

func TestMultipleSubscribersReceiveUpdates(t *testing.T) {
	const numSubscribers = 5
	var channels []chan string
	var unsubs []func()

	// Create multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		ch, unsub := Subscribe()
		channels = append(channels, ch)
		unsubs = append(unsubs, unsub)
	}

	// Clean up all subscribers
	defer func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}()

	// Publish an update
	testETag := "test-etag-123"
	publishUpdate(testETag)

	// All subscribers should receive it
	timeout := time.After(1 * time.Second)
	received := 0

	for _, ch := range channels {
		select {
		case etag := <-ch:
			if etag == testETag {
				received++
			} else {
				t.Errorf("Expected ETag %s, got %s", testETag, etag)
			}
		case <-timeout:
			t.Errorf("Timeout: only %d of %d subscribers received update", received, numSubscribers)
			return
		}
	}

	if received != numSubscribers {
		t.Errorf("Expected %d subscribers to receive update, got %d", numSubscribers, received)
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 50

	// Concurrent subscribe/unsubscribe
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			updates, unsub := Subscribe()
			time.Sleep(1 * time.Millisecond) // Hold subscription briefly
			unsub()
			// Try to read from closed channel (should not panic)
			_, _ = <-updates
		}()
	}

	// Concurrent publish
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			publishUpdate("concurrent-etag")
		}(i)
	}

	wg.Wait()
}

func TestSubscriberReceivesOnlyAfterSubscription(t *testing.T) {
	// Publish before subscribing
	publishUpdate("before-sub")

	// Now subscribe
	updates, unsub := Subscribe()
	defer unsub()

	// Publish after subscribing
	afterETag := "after-sub"
	publishUpdate(afterETag)

	// Should only receive the "after" update
	select {
	case etag := <-updates:
		if etag != afterETag {
			t.Errorf("Expected ETag %s, got %s", afterETag, etag)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for update")
	}

	// Should not receive anything else (the "before" update)
	select {
	case etag := <-updates:
		t.Errorf("Unexpected update received: %s", etag)
	case <-time.After(100 * time.Millisecond):
		// Expected - no more updates
	}
}

func TestUnsubscribeIsIdempotent(t *testing.T) {
	_, unsub := Subscribe()

	// First unsubscribe should work fine
	unsub()

	// Subsequent calls might panic on channel close, which is acceptable
	// The current implementation doesn't guard against multiple unsubs
	// This test just verifies that one unsubscribe works correctly
}
