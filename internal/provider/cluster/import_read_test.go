package cluster

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// An imported cluster starts from empty state and gets only what Read stores.
// The next plan must not replace it because of alternator_write_isolation.
func TestAlternatorWriteIsolationPlan(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name           string
		api, isolation string // what the API returns
		config         string // alternator_write_isolation in config, "" for unset
		wantDiff       bool
	}{
		{name: "imported CQL cluster", api: "CQL"},
		{name: "CQL cluster, value set in config", api: "CQL", config: "always_use_lwt"},
		{name: "imported Alternator cluster", api: "ALTERNATOR", isolation: "always_use_lwt", config: "always_use_lwt"},
		{name: "imported Alternator cluster, API has no value", api: "ALTERNATOR"},
		{name: "Alternator cluster, value changed", api: "ALTERNATOR", isolation: "only_rmw_uses_lwt", config: "always_use_lwt", wantDiff: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := ResourceCluster()
			d := r.TestResourceData()
			d.SetId("3")
			cluster := &model.Cluster{
				ID:                       3,
				UserAPIInterface:         tt.api,
				AlternatorWriteIsolation: tt.isolation,
				Status:                   "ACTIVE",
				Region:                   &model.CloudProviderRegion{ExternalID: "us-east-1"},
				ScyllaVersion:            &model.ScyllaVersion{Version: "2025.1"},
				Datacenter:               &model.Datacenter{Name: "AWS_US_EAST_1", CIDRBlock: "172.31.0.0/16"},
				Datacenters:              []model.Datacenter{{}},
			}
			require.NoError(t, setClusterKVs(d, cluster, "AWS", "", "", nil, &scylla.CloudProvider{}))

			config := map[string]any{"user_api_interface": tt.api, "node_type": "i8g.large", "min_nodes": 3}
			if tt.config != "" {
				config["alternator_write_isolation"] = tt.config
			}
			diff, err := r.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(config), nil)
			require.NoError(t, err)

			// The test config sets only a few attributes, so the diff lists
			// others too. Check only whether this one changes.
			var got bool
			if diff != nil {
				a := diff.Attributes["alternator_write_isolation"]
				got = a != nil && (a.Old != a.New || a.NewRemoved)
			}
			require.Equal(t, tt.wantDiff, got)
		})
	}
}
