//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Defaults from cmd/server/main.go: RATE_LIMIT_BURST=10, authed=100rps. The
// docker-compose service in this repo does not override these, so the
// authenticated bucket trips on the (burst+1)th rapid call from the same
// tenant on the same method.
const ratelimitBurst = 10

// burstUntilExhausted hammers fn until it returns ResourceExhausted, up to
// burst*3 attempts. Returns the number of successful calls before the trip
// and the final ResourceExhausted error. Fails the test if some other error
// is returned or if the limiter never trips.
func burstUntilExhausted(t *testing.T, fn func() error) (successes int, rlErr error) {
	t.Helper()
	for i := 0; i < ratelimitBurst*3; i++ {
		err := fn()
		if err == nil {
			successes++
			continue
		}
		if status.Code(err) == codes.ResourceExhausted {
			return successes, err
		}
		t.Fatalf("unexpected error at iteration %d (after %d successes): %v", i, successes, err)
	}
	t.Fatalf("rate limiter never tripped after %d successes", successes)
	return
}

// TestRateLimit_ExhaustsAuthedBucket: an authenticated client hammering one
// method on one tenant trips the limiter after ~burst calls.
func TestRateLimit_ExhaustsAuthedBucket(t *testing.T) {
	fixture := bootstrapMatrixFixture(t, "rl-exhaust")
	conn := dial(t)
	c := scopedClients(t, conn, roleAdmin, fixture.tenantID)
	ctx := context.Background()

	successes, err := burstUntilExhausted(t, func() error {
		_, e := c.admin.GetSchema(ctx, fixture.schemaID)
		return e
	})

	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	// Burst is the upper bound on successes-before-trip; replenishment may
	// add a few more during the loop, but we should always see at least
	// burst-1 successes (allow a small slack for clock drift).
	assert.GreaterOrEqual(t, successes, ratelimitBurst-1, "should have allowed at least burst-1 requests through")
}

// TestRateLimit_PerTenantIsolation: exhausting tenant A does not affect
// tenant B — each authenticated bucket is keyed by tenant.
func TestRateLimit_PerTenantIsolation(t *testing.T) {
	a := bootstrapMatrixFixture(t, "rl-tenant-a")
	b := bootstrapMatrixFixture(t, "rl-tenant-b")
	conn := dial(t)
	cA := scopedClients(t, conn, roleAdmin, a.tenantID)
	cB := scopedClients(t, conn, roleAdmin, b.tenantID)
	ctx := context.Background()

	_, _ = burstUntilExhausted(t, func() error {
		_, e := cA.admin.GetSchema(ctx, a.schemaID)
		return e
	})

	// Tenant B has its own bucket and must not be blocked.
	_, err := cB.admin.GetSchema(ctx, b.schemaID)
	require.NoError(t, err, "tenant B should not be affected by tenant A exhaustion")
}

// TestRateLimit_PerMethodIsolation: exhausting one method on a tenant does
// not affect a different method on the same tenant — buckets are keyed by
// (tenant, method).
func TestRateLimit_PerMethodIsolation(t *testing.T) {
	f := bootstrapMatrixFixture(t, "rl-method")
	conn := dial(t)
	c := scopedClients(t, conn, roleAdmin, f.tenantID)
	ctx := context.Background()

	_, _ = burstUntilExhausted(t, func() error {
		_, e := c.admin.GetSchema(ctx, f.schemaID)
		return e
	})

	// ListTenants is a different method — its bucket is independent. We
	// don't care whether the call returns NotFound / OK / something else;
	// only that ResourceExhausted is NOT returned.
	_, err := c.admin.ListTenants(ctx, f.schemaID)
	if err != nil && status.Code(err) == codes.ResourceExhausted {
		t.Fatalf("ListTenants was rate-limited despite being a different method bucket: %v", err)
	}
}
