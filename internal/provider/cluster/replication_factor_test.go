package cluster

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/stretchr/testify/require"
)

// existingClusterState is the state of a Standard cluster. A config with the
// same values plans no replacement. When the SDK plans a replacement, it runs
// CustomizeDiff again without state, which hides what the state does. rf is
// empty for state written before replication_factor existed.
func existingClusterState(rf, minNodes string) *terraform.InstanceState {
	attrs := map[string]string{
		"id":                            "42",
		"name":                          "cluster",
		"cloud":                         "AWS",
		"region":                        "us-east-1",
		"node_type":                     "i3.large",
		"node_disk_size":                "468",
		"min_nodes":                     minNodes,
		"user_api_interface":            "CQL",
		"enable_dns":                    "true",
		"enable_vpc_peering":            "true",
		"backup_retention_days":         "1",
		"cidr_block":                    "172.31.0.0/16",
		"scylla_version":                "2025.1",
		"availability_zone_ids.#":       "0",
		"encryption_at_rest.#":          "1",
		"encryption_at_rest.0.enabled":  "true",
		"encryption_at_rest.0.key_id":   "key-deadbeef",
		"encryption_at_rest.0.provider": "scylla-aws",
	}
	if rf != "" {
		attrs["replication_factor"] = rf
	}
	return &terraform.InstanceState{ID: "42", Attributes: attrs}
}

// TestReplicationFactorPlan drives the real diff path, including CustomizeDiff.
func TestReplicationFactorPlan(t *testing.T) {
	t.Parallel()

	config := func(rf cty.Value) map[string]cty.Value {
		return map[string]cty.Value{
			"name":               cty.StringVal("cluster"),
			"cloud":              cty.StringVal("AWS"),
			"region":             cty.StringVal("us-east-1"),
			"node_type":          cty.StringVal("i3.large"),
			"min_nodes":          cty.NumberIntVal(6),
			"replication_factor": rf,
		}
	}

	existing := func(rf string) *terraform.InstanceState {
		return existingClusterState(rf, "6")
	}

	// change returns the replication_factor diff, or nil if the plan does not
	// change it. The SDK also lists attributes whose old and new values match.
	change := func(diff *terraform.InstanceDiff) *terraform.ResourceAttrDiff {
		if diff == nil {
			return nil
		}
		attr := diff.Attributes["replication_factor"]
		if attr == nil || (!attr.NewComputed && attr.Old == attr.New) {
			return nil
		}
		return attr
	}

	t.Run("new cluster without a value plans it as known after apply", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, nil, config(cty.NullVal(cty.Number)))
		require.NoError(t, err)

		attr := diff.Attributes["replication_factor"]
		require.NotNil(t, attr)
		require.True(t, attr.NewComputed)
	})

	t.Run("new cluster with a value plans that value", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, nil, config(cty.NumberIntVal(2)))
		require.NoError(t, err)

		attr := diff.Attributes["replication_factor"]
		require.NotNil(t, attr)
		require.Equal(t, "2", attr.New)
	})

	t.Run("the base state plans no replacement", func(t *testing.T) {
		t.Parallel()

		// The other subtests depend on this. Without it, a replacement
		// planned for another attribute hides what replication_factor does.
		diff, err := clusterDiff(t, existing("3"), config(cty.NullVal(cty.Number)))
		require.NoError(t, err)
		require.False(t, diff != nil && diff.RequiresNew())
	})

	t.Run("no value in config keeps the value in state", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, existing("2"), config(cty.NullVal(cty.Number)))
		require.NoError(t, err)
		require.Nil(t, change(diff))
	})

	t.Run("same value is no change", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, existing("3"), config(cty.NumberIntVal(3)))
		require.NoError(t, err)
		require.Nil(t, change(diff))
	})

	t.Run("a change replaces the cluster", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, existing("3"), config(cty.NumberIntVal(2)))
		require.NoError(t, err)

		attr := change(diff)
		require.NotNil(t, attr)
		require.Equal(t, "3", attr.Old)
		require.Equal(t, "2", attr.New)
		require.True(t, attr.RequiresNew, "the API can set the replication factor only at creation")
	})

	t.Run("a value against state without one does not replace the cluster", func(t *testing.T) {
		t.Parallel()

		// Old state does not tell the real value. Replacing here could
		// destroy a cluster that already has the configured value.
		diff, err := clusterDiff(t, existing(""), config(cty.NumberIntVal(3)))
		require.NoError(t, err)
		require.Nil(t, change(diff))
		require.False(t, diff != nil && diff.RequiresNew())
	})
}

func TestSetClusterKVsSetsReplicationFactor(t *testing.T) {
	t.Parallel()

	cluster := func(rf int64) *model.Cluster {
		return &model.Cluster{
			ID:                123,
			ClusterName:       "cluster",
			UserAPIInterface:  "CQL",
			Status:            "ACTIVE",
			Region:            &model.CloudProviderRegion{ExternalID: "us-east-1"},
			ScyllaVersion:     &model.ScyllaVersion{Version: "2025.1"},
			Datacenter:        &model.Datacenter{Name: "AWS_US_EAST_1"},
			Datacenters:       []model.Datacenter{{}},
			ReplicationFactor: rf,
		}
	}

	t.Run("reads the value from the API", func(t *testing.T) {
		t.Parallel()

		data := ResourceCluster().TestResourceData()
		require.NoError(t, setClusterKVs(data, cluster(2), "AWS", "", "", nil, &scylla.CloudProvider{}))
		require.Equal(t, 2, data.Get("replication_factor"))
	})

	t.Run("keeps the state when the API reports no value", func(t *testing.T) {
		t.Parallel()

		data := ResourceCluster().TestResourceData()
		require.NoError(t, data.Set("replication_factor", 2))
		require.NoError(t, setClusterKVs(data, cluster(0), "AWS", "", "", nil, &scylla.CloudProvider{}))
		require.Equal(t, 2, data.Get("replication_factor"))
	})
}
