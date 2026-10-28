package cluster

import (
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func TestReplicationFactorSchema(t *testing.T) {
	t.Parallel()
	s := ResourceCluster().Schema["replication_factor"]
	require.True(t, s.Optional)
	require.True(t, s.Computed)
	require.True(t, s.ForceNew)
	require.Nil(t, s.Default, "a default would diff against state written by older providers")
	require.True(t, s.ValidateDiagFunc(0, cty.Path{}).HasError())
	require.False(t, s.ValidateDiagFunc(3, cty.Path{}).HasError())
}

func TestReplicationFactorDefault(t *testing.T) {
	t.Parallel()
	r := ResourceCluster()
	require.Equal(t, int64(3), replicationFactor(r.TestResourceData()))

	d := r.TestResourceData()
	require.NoError(t, d.Set("replication_factor", 2))
	require.Equal(t, int64(2), replicationFactor(d))
}

// rfDiff diffs an existing cluster in state against a configuration
// that adds extra to the base attributes.
func rfDiff(t *testing.T, stateExtra map[string]string, cfgExtra map[string]interface{}) *terraform.InstanceDiff {
	t.Helper()
	attrs := map[string]string{
		"id": "3", "cluster_id": "3", "name": "c", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": "3", "cidr_block": "172.31.0.0/16",
		"enable_dns": "true", "enable_vpc_peering": "true", "user_api_interface": "CQL",
		"alternator_write_isolation": "only_rmw_uses_lwt",
		"availability_zone_ids.#":    "1", "availability_zone_ids.0": "use1-az1",
		"encryption_at_rest.#": "1", "encryption_at_rest.0.enabled": "true", "encryption_at_rest.0.provider": "scylla-aws",
	}
	for k, v := range stateExtra {
		attrs[k] = v
	}
	cfg := map[string]interface{}{
		"name": "c", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": 3, "cidr_block": "172.31.0.0/16",
	}
	for k, v := range cfgExtra {
		cfg[k] = v
	}
	state := &terraform.InstanceState{ID: "3", Attributes: attrs, Meta: map[string]interface{}{"schema_version": "3"}}
	diff, err := ResourceCluster().Diff(context.Background(), state, terraform.NewResourceConfigRaw(cfg), nil)
	require.NoError(t, err)
	return diff
}

func TestReplicationFactorUpgradeHasNoDiff(t *testing.T) {
	t.Parallel()
	// State written before replication_factor existed, config not setting it.
	diff := rfDiff(t, nil, nil)
	if diff != nil {
		_, changed := diff.Attributes["replication_factor"]
		require.False(t, changed, "unexpected diff: %v", diff.Attributes["replication_factor"])
		require.False(t, diff.RequiresNew())
	}
}

func TestReplicationFactorChangeReplaces(t *testing.T) {
	t.Parallel()
	diff := rfDiff(t, map[string]string{"replication_factor": "3"}, map[string]interface{}{"replication_factor": 2})
	require.NotNil(t, diff)
	require.True(t, diff.RequiresNew())
	require.True(t, diff.Attributes["replication_factor"].RequiresNew)
}

func TestSetClusterKVsSetsReplicationFactor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		api  int64
		want int
	}{{3, 3}, {2, 2}, {0, 0}} {
		d := ResourceCluster().TestResourceData()
		cluster := &model.Cluster{
			ID:                3,
			ReplicationFactor: tc.api,
			Status:            "ACTIVE",
			Region:            &model.CloudProviderRegion{ExternalID: "us-east-1"},
			ScyllaVersion:     &model.ScyllaVersion{Version: "2025.1"},
			Datacenter:        &model.Datacenter{Name: "AWS_US_EAST_1", CIDRBlock: "172.31.0.0/16"},
			Datacenters:       []model.Datacenter{{}},
		}
		require.NoError(t, setClusterKVs(d, cluster, "AWS", "", "", nil, &scylla.CloudProvider{}))
		require.Equal(t, tc.want, d.Get("replication_factor"), "api %d", tc.api)
	}
}
