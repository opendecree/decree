//go:build e2e

package e2e

import "github.com/opendecree/decree/sdk/configclient"

// noVer drops the *ConfigVersion returned by configclient write methods so the
// resulting error can be asserted directly in tests:
//
//	require.NoError(t, noVer(cfg.Set(ctx, tenant.ID, "k", "v")))
func noVer(_ *configclient.ConfigVersion, err error) error { return err }
