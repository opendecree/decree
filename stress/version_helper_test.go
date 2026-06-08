//go:build stress

package stress

import "github.com/opendecree/decree/sdk/configclient"

// noVer drops the *ConfigVersion returned by configclient write methods so the
// resulting error can be asserted directly in stress tests.
func noVer(_ *configclient.ConfigVersion, err error) error { return err }
