package cluster

import "testing"

func TestHistoricalSchemasInternalValidate(t *testing.T) {
	t.Parallel()
	for name, r := range map[string]func() error{
		"v0": func() error { return resourceClusterV0().InternalValidate(nil, true) },
		"v1": func() error { return resourceClusterV1().InternalValidate(nil, true) },
		"v2": func() error { return resourceClusterV2().InternalValidate(nil, true) },
	} {
		if err := r(); err != nil {
			t.Errorf("cluster schema %s is invalid: %s", name, err)
		}
	}
}
