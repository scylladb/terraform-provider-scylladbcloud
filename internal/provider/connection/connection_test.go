package connection

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"
)

func TestStatusValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		status  string
		wantErr bool
	}{
		{status: "ACTIVE"},
		{status: "INACTIVE"},
		{status: "INACTIVE ", wantErr: true},
		{status: "active", wantErr: true},
		{status: "DELETED", wantErr: true},
	} {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()
			config := terraform.NewResourceConfigRaw(map[string]any{
				"cluster_id": 3,
				"datacenter": "AWS_US_EAST_1",
				"type":       "AWS_TGW_ATTACHMENT",
				"cidrlist":   []any{"10.0.0.0/16"},
				"data":       map[string]any{"tgwid": "tgw-1", "ramarn": "arn:aws:ram::1:resource-share/1"},
				"status":     tt.status,
			})

			diags := ResourceClusterConnection().Validate(config)

			if !tt.wantErr {
				require.False(t, diags.HasError(), "%v", diags)
				return
			}
			// Only the status may be wrong, so the only error is about it.
			require.Len(t, diags, 1, "%v", diags)
			require.Contains(t, diags[0].Summary, "expected status to be one of")
		})
	}
}

// Until a status change takes effect, the API can still return the old
// status. The wait must keep waiting then, not fail.
func TestConnectionPendingStatuses(t *testing.T) {
	t.Parallel()
	require.Contains(t, connectionPendingStatuses("INACTIVE"), "ACTIVE")
	require.NotContains(t, connectionPendingStatuses("INACTIVE"), "INACTIVE")
	require.Contains(t, connectionPendingStatuses("ACTIVE"), "INACTIVE")
	require.NotContains(t, connectionPendingStatuses("ACTIVE"), "ACTIVE")
	require.ElementsMatch(t, []string{"PENDING", "INIT", "DELETING"}, connectionPendingStatuses("DELETED"))
}
