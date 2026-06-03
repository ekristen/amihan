package apiserver

import (
	"sync"
	"time"
)

// dispatchTracker records the last time a given alert key triggered a dispatch
// so repeated webhook deliveries for the same firing alert don't re-trigger the
// Nomad job until a cooldown has elapsed.
type dispatchTracker struct {
	mu        sync.Mutex
	cooldown  time.Duration
	last      map[string]time.Time
	lastPrune time.Time
}

func newDispatchTracker(cooldown time.Duration) *dispatchTracker {
	return &dispatchTracker{
		cooldown: cooldown,
		last:     make(map[string]time.Time),
	}
}

// allow reports whether key may be dispatched at now. When it returns true it
// records now as the latest dispatch time. When it returns false it also returns
// how long until the key becomes eligible again.
func (t *dispatchTracker) allow(key string, now time.Time) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneLocked(now)

	if last, ok := t.last[key]; ok {
		if elapsed := now.Sub(last); elapsed < t.cooldown {
			return false, t.cooldown - elapsed
		}
	}

	t.last[key] = now
	return true, 0
}

// forget removes a key so the next delivery is eligible immediately. Used when a
// dispatch attempt fails and we want to allow a prompt retry rather than blocking
// for the full cooldown.
func (t *dispatchTracker) forget(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, key)
}

// pruneLocked drops entries older than the cooldown to bound memory growth. It is
// rate-limited to run at most once per cooldown window. The caller must hold the
// lock.
func (t *dispatchTracker) pruneLocked(now time.Time) {
	if now.Sub(t.lastPrune) < t.cooldown {
		return
	}
	t.lastPrune = now
	for k, ts := range t.last {
		if now.Sub(ts) >= t.cooldown {
			delete(t.last, k)
		}
	}
}
