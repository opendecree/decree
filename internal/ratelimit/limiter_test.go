package ratelimit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/opendecree/decree/internal/ratelimit"
)

// TestInProcessLimiter_LRUEviction verifies that inserting cap+1 unique keys evicts
// the least-recently-used bucket, and the evicted key gets a fresh full bucket on reuse.
func TestInProcessLimiter_LRUEviction(t *testing.T) {
	const cap = 3
	// rate=0: no token replenishment; burst=1: one token per fresh bucket.
	lim := ratelimit.NewInProcess(rate.Limit(0), 1, ratelimit.WithMaxBuckets(cap))

	// Fill to capacity; each call consumes the sole token.
	// After three inserts, LRU order (back→front): key-0, key-1, key-2.
	lim.Allow("key-0")
	lim.Allow("key-1")
	lim.Allow("key-2")

	// Inserting key-3 evicts key-0 (LRU).
	lim.Allow("key-3")

	// key-0 was evicted; requesting it yields a fresh bucket with 1 token.
	assert.True(t, lim.Allow("key-0"), "evicted key should get a fresh bucket")
	assert.False(t, lim.Allow("key-0"), "fresh bucket has only one token")
}
