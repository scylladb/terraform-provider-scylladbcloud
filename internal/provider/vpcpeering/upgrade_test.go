package vpcpeering

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgradeV0(t *testing.T) {
	t.Parallel()
	for name, tt := range map[string]struct {
		raw  string
		want []any
	}{
		"single block":   {raw: `{"id":"pcx-1","peer_cidr_block":"10.1.0.0/16"}`, want: []any{"10.1.0.0/16"}},
		"comma list":     {raw: `{"id":"pcx-1","peer_cidr_block":"10.1.0.0/16, 10.2.0.0/16,"}`, want: []any{"10.1.0.0/16", "10.2.0.0/16"}},
		"already a list": {raw: `{"id":"pcx-1","peer_cidr_blocks":["10.3.0.0/16"]}`, want: []any{"10.3.0.0/16"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var raw map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(tt.raw), &raw))

			got, err := resourceVPCPeeringUpgradeV0(context.Background(), raw, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, got["peer_cidr_blocks"])
			require.NotContains(t, got, "peer_cidr_block")
		})
	}
}
