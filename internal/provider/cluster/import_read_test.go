package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// An imported cluster starts from empty state, so Read alone must store every
// attribute a plan compares, or the next plan diffs.
func TestReadStoresAlternatorWriteIsolation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ api, isolation, want string }{
		{api: "CQL", want: "only_rmw_uses_lwt"},
		{api: "ALTERNATOR", isolation: "always_use_lwt", want: "always_use_lwt"},
	} {
		d := ResourceCluster().TestResourceData()
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
		require.Equal(t, tt.want, d.State().Attributes["alternator_write_isolation"], tt.api)
	}
}
