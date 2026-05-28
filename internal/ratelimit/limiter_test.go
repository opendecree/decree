package ratelimit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/opendecree/decree/internal/ratelimit"
)

// TestInProcessLimiter_LRUEviction verifies that inserting cap+1 unique keys evicts
// the least-recently-used bucket, and that the evicted key is quarantined on re-entry
// (starts with zero tokens, no free burst).
func TestInProcessLimiter_LRUEviction(t *testing.T) {
	const cap = 3
	// rate=0: no token replenishment; burst=1: one token per fresh bucket.
	lim := ratelimit.NewInProcess(rate.Limit(0), 1, ratelimit.WithMaxBuckets(cap))

	// Fill to capacity; each call consumes the sole token.
	// After three inserts, LRU order (back→front): key-0, key-1, key-2.
	lim.Allow("key-0")
	lim.Allow("key-1")
	lim.Allow("key-2")

	// Inserting key-3 evicts key-0 (LRU) into quarantine.
	lim.Allow("key-3")

	// key-0 re-enters from quarantine: burst is drained, so the first call is denied.
	assert.False(t, lim.Allow("key-0"), "quarantined key must not get a free burst on re-entry")
}

// TestInProcessLimiter_QuarantineBlocksFreeBurst verifies the security property:
// cycling keys through LRU eviction cannot be used to reset token buckets.
func TestInProcessLimiter_QuarantineBlocksFreeBurst(t *testing.T) {
	const cap = 2
	// burst=5, rate=0 (no refill): an attacker hoping to cycle keys gets no extra tokens.
	lim := ratelimit.NewInProcess(rate.Limit(0), 5, ratelimit.WithMaxBuckets(cap))

	// Consume all tokens for key-a.
	for range 5 {
		lim.Allow("key-a")
	}
	assert.False(t, lim.Allow("key-a"), "key-a should be exhausted")

	// Evict key-a by filling the cache with other keys.
	lim.Allow("key-b")
	lim.Allow("key-c") // key-a is now evicted (LRU)

	// key-a re-enters: quarantine must prevent the free burst.
	assert.False(t, lim.Allow("key-a"), "evicted key must not regain burst tokens via re-entry")
}

// TestInProcessLimiter_NonEvictedKeyGetsFreshBucket verifies that a key that was never
// evicted (i.e. not in quarantine) still gets a normal full bucket on first use.
func TestInProcessLimiter_NonEvictedKeyGetsFreshBucket(t *testing.T) {
	lim := ratelimit.NewInProcess(rate.Limit(0), 1, ratelimit.WithMaxBuckets(10))
	assert.True(t, lim.Allow("fresh-key"), "unseen key should get a full burst")
	assert.False(t, lim.Allow("fresh-key"), "burst of 1 should be exhausted")
}

// TestInProcessLimiter_QuarantineLogOverflow verifies that the quarantine log itself
// evicts its oldest entry when it reaches capacity, allowing that key to regain a
// fresh burst as if it had never been seen.
func TestInProcessLimiter_QuarantineLogOverflow(t *testing.T) {
	const cap = 2
	lim := ratelimit.NewInProcess(rate.Limit(0), 1, ratelimit.WithMaxBuckets(cap))

	// Fill and overflow the quarantine (cap=2) by evicting three distinct keys.
	// Sequence (main LRU cap=2):
	//   Allow a, b       → main: [b, a]
	//   Allow c          → evicts a → quarantine: [a]
	//   Allow d          → evicts b → quarantine: [b, a]  (full)
	//   Allow e          → evicts c → quarantine: [c, b]  (a evicted from quarantine)
	lim.Allow("key-a")
	lim.Allow("key-b")
	lim.Allow("key-c") // evicts key-a into quarantine
	lim.Allow("key-d") // evicts key-b into quarantine (quarantine full)
	lim.Allow("key-e") // evicts key-c; key-a rolls off the quarantine

	// key-a is no longer quarantined — it should receive a fresh full burst.
	assert.True(t, lim.Allow("key-a"), "key-a rolled off quarantine and should get a fresh burst")
}

// TestInProcessLimiter_QuarantineDuplicateEviction verifies that evicting an
// already-quarantined key is a no-op (no duplicate entry, key stays quarantined).
func TestInProcessLimiter_QuarantineDuplicateEviction(t *testing.T) {
	const cap = 3
	lim := ratelimit.NewInProcess(rate.Limit(0), 1, ratelimit.WithMaxBuckets(cap))

	// Evict key-a into quarantine.
	lim.Allow("key-a")
	lim.Allow("key-b")
	lim.Allow("key-c")
	lim.Allow("key-d") // evicts key-a → quarantine: [a]

	// key-a re-enters main from quarantine (burst drained; still in quarantine).
	lim.Allow("key-a") // evicts key-b → quarantine: [b, a]

	// Touch key-d and key-c to push key-a to the back of the main LRU.
	lim.Allow("key-d")
	lim.Allow("key-c")

	// Adding key-e evicts key-a from main again; quarantine() must no-op (a already present).
	lim.Allow("key-e")

	// key-a is still quarantined — no free burst.
	assert.False(t, lim.Allow("key-a"), "doubly-evicted key must remain quarantined")
}
