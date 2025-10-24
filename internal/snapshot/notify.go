package snapshot

import (
	"sync"
)

type subCh = chan string // carries new ETags

var (
	mu   sync.Mutex
	subs = make(map[subCh]struct{})
)

// Subscribe registers a listener and returns its channel and an unsubscribe func.
func Subscribe() (subCh, func()) {
	ch := make(subCh, 1)
	mu.Lock()
	subs[ch] = struct{}{}
	mu.Unlock()

	unsub := func() {
		mu.Lock()
		delete(subs, ch)
		close(ch)
		mu.Unlock()
	}
	return ch, unsub
}

// publishUpdate notifies all listeners (non-blocking).
func publishUpdate(etag string) {
	mu.Lock()
	for ch := range subs {
		select {
		case ch <- etag:
		default: // if client is slow, skip instead of blocking
		}
	}
	mu.Unlock()
}
