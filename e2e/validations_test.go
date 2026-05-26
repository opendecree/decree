//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// crossFieldYAML defines a schema with one CEL validation rule
// (payments.min_amount < payments.max_amount) and one dependentRequired
// entry (refunds_enabled → refund_window). The CEL engine and the
// dependentRequired carve-out are independent — both must hold for a
// write to land.
var crossFieldYAML = []byte(`spec_version: "v1"
name: cross-field-e2e
description: Cross-field validation showcase
fields:
  payments.min_amount:
    type: number
  payments.max_amount:
    type: number
  payments.refunds_enabled:
    type: bool
  payments.refund_window:
    type: duration
    nullable: true
validations:
  - path: payments
    rule: "self.payments.min_amount == null || self.payments.max_amount == null || self.payments.min_amount < self.payments.max_amount"
    message: "payments.min_amount must be less than payments.max_amount"
    reason: MIN_LESS_THAN_MAX
dependentRequired:
  payments.refunds_enabled:
    - payments.refund_window
`)

// TestValidations_CrossFieldRule_RejectsAndAccepts exercises the CEL engine
// end-to-end: ImportSchema accepts the rule, SetField returns
// InvalidArgument with the rule's message when violated, and SetField
// succeeds once the state satisfies the rule.
func TestValidations_CrossFieldRule_RejectsAndAccepts(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	imported, err := admin.ImportSchema(ctx, crossFieldYAML)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteSchema(ctx, imported.ID) })

	_, err = admin.PublishSchema(ctx, imported.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "cross-field-tenant-e2e", imported.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteTenant(ctx, tenant.ID) })

	// Stage min and max in two separate writes — order matters only because
	// SetField evaluates each write in isolation; setting min above max
	// produces a violation on the SECOND write whichever order we use.
	require.NoError(t, cfg.SetFloat(ctx, tenant.ID, "payments.max_amount", 100))

	// Violating write: min equals max.
	err = cfg.SetFloat(ctx, tenant.ID, "payments.min_amount", 100)
	require.Error(t, err, "rule must fire when min == max")
	assert.ErrorIs(t, err, configclient.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "payments.min_amount must be less than payments.max_amount")

	// Passing write: min strictly less than max.
	require.NoError(t, cfg.SetFloat(ctx, tenant.ID, "payments.min_amount", 10))

	// Confirm both values landed.
	all, err := cfg.GetAll(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "10", all["payments.min_amount"])
	assert.Equal(t, "100", all["payments.max_amount"])
}

// TestValidations_DependentRequiredAlongsideCEL verifies that the
// dependentRequired carve-out fires inside the same schema that declares a
// CEL rule. Both run inside the same write transaction; either firing
// rejects the write.
func TestValidations_DependentRequiredAlongsideCEL(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	imported, err := admin.ImportSchema(ctx, crossFieldYAML)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteSchema(ctx, imported.ID) })

	_, err = admin.PublishSchema(ctx, imported.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "dep-required-tenant-e2e", imported.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteTenant(ctx, tenant.ID) })

	// Pre-seed valid min/max so the CEL rule cannot fire and obscure the
	// dependentRequired failure.
	require.NoError(t, cfg.SetFloat(ctx, tenant.ID, "payments.min_amount", 1))
	require.NoError(t, cfg.SetFloat(ctx, tenant.ID, "payments.max_amount", 100))

	// Enabling refunds without setting the refund_window violates
	// dependentRequired.
	err = cfg.SetBool(ctx, tenant.ID, "payments.refunds_enabled", true)
	require.Error(t, err)
	assert.ErrorIs(t, err, configclient.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "payments.refund_window")
}

// TestValidations_SetFields_PostMergeStateChecked verifies that a
// multi-field write evaluates the CEL rule against the FINAL state — an
// intermediate violation is tolerated.
func TestValidations_SetFields_PostMergeStateChecked(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	imported, err := admin.ImportSchema(ctx, crossFieldYAML)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteSchema(ctx, imported.ID) })

	_, err = admin.PublishSchema(ctx, imported.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "setfields-tenant-e2e", imported.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteTenant(ctx, tenant.ID) })

	// SetManyTyped writes min=10 and max=100 in the same transaction. The
	// final state satisfies min < max even though min would temporarily
	// equal max if the writes were ordered as two separate SetField calls
	// after a prior write at min=5.
	err = cfg.SetManyTyped(ctx, tenant.ID, map[string]*configclient.TypedValue{
		"payments.min_amount": configclient.FloatVal(10),
		"payments.max_amount": configclient.FloatVal(100),
	}, "")
	require.NoError(t, err)

	all, err := cfg.GetAll(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "10", all["payments.min_amount"])
	assert.Equal(t, "100", all["payments.max_amount"])
}

// TestValidations_RejectsMalformedRuleAtImport checks the lint path — a
// schema with a syntactically broken CEL rule must be rejected at
// ImportSchema rather than failing one user later.
func TestValidations_RejectsMalformedRuleAtImport(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	badYAML := []byte(`spec_version: "v1"
name: bad-cel-e2e
fields:
  payments.amount:
    type: number
validations:
  - rule: "self.payments.amount <"
    message: "broken"
`)

	_, err := admin.ImportSchema(ctx, badYAML)
	require.Error(t, err)
	assert.ErrorIs(t, err, adminclient.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "validations[0]")
}
