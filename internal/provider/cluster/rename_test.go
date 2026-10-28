package cluster

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestClusterNameIsUpdatable(t *testing.T) {
	t.Parallel()
	s := ResourceCluster().Schema["name"]
	require.False(t, s.ForceNew)
	require.True(t, s.Required)
}

// renamedClusterData is a cluster in state as "old-name" whose configuration
// renames it, as Terraform hands it to Update.
func renamedClusterData(t *testing.T) *schema.ResourceData {
	t.Helper()
	r := ResourceCluster()
	attrs := map[string]string{
		"id": "3", "cluster_id": "3", "name": "old-name", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": "3", "cidr_block": "172.31.0.0/16",
		"enable_dns": "true", "enable_vpc_peering": "true", "user_api_interface": "CQL",
		"alternator_write_isolation": "only_rmw_uses_lwt",
		"availability_zone_ids.#":    "1", "availability_zone_ids.0": "use1-az1",
		"encryption_at_rest.#": "1", "encryption_at_rest.0.enabled": "true", "encryption_at_rest.0.provider": "scylla-aws",
	}
	state := &terraform.InstanceState{ID: "3", Attributes: attrs, Meta: map[string]interface{}{"schema_version": "3"}}
	cfg := map[string]interface{}{
		"name": "new-name", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": 3, "cidr_block": "172.31.0.0/16",
	}
	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(cfg), nil)
	require.NoError(t, err)
	require.False(t, diff.RequiresNew(), "a rename must not replace the cluster")
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	require.NoError(t, err)
	require.True(t, d.HasChange("name"))
	require.False(t, d.HasChange("min_nodes"))
	require.False(t, d.HasChange("scaling"))
	return d
}

func TestResourceClusterUpdateRenames(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})
	srv.Reply(http.MethodPatch, "/account/1/cluster/3/name", model.ClusterRequest{ID: 50, Status: "QUEUED"})
	srv.Sequence(http.MethodGet, "/account/1/cluster/request/50",
		scyllatest.Response{Data: model.ClusterRequest{ID: 50, Status: "IN_PROGRESS"}},
		scyllatest.Response{Data: model.ClusterRequest{ID: 50, Status: "COMPLETED"}},
	)

	diags := resourceClusterUpdate(context.Background(), renamedClusterData(t), srv.Client(t))
	require.False(t, diags.HasError(), "%v", diags)

	calls := srv.CallsTo(http.MethodPatch, "/account/1/cluster/3/name")
	require.Len(t, calls, 1)
	var body map[string]string
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]string{"name": "new-name"}, body)
	require.Len(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/request/50"), 2)
}

func TestResourceClusterUpdateRenameFails(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})
	srv.Reply(http.MethodPatch, "/account/1/cluster/3/name", model.ClusterRequest{ID: 50, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/50", model.ClusterRequest{ID: 50, Status: "FAILED"})

	diags := resourceClusterUpdate(context.Background(), renamedClusterData(t), srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, `failed to rename the cluster 3 to "new-name"`)
}

func TestResourceClusterUpdateRenameRejected(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})
	srv.Fail(http.MethodPatch, "/account/1/cluster/3/name", http.StatusOK, "040001")

	diags := resourceClusterUpdate(context.Background(), renamedClusterData(t), srv.Client(t))
	require.True(t, diags.HasError())
	require.Empty(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/request/50"))
}
