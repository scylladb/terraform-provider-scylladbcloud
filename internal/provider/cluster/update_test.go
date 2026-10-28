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

// updateData is cluster 3 in state as "old-name" in mode "both", handed to
// Update with a configuration that renames it and encrypts client traffic.
func updateData(t *testing.T) *schema.ResourceData {
	t.Helper()
	r := ResourceCluster()
	state := &terraform.InstanceState{ID: "3", Meta: map[string]interface{}{"schema_version": "3"}, Attributes: map[string]string{
		"id": "3", "cluster_id": "3", "name": "old-name", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": "3", "cidr_block": "172.31.0.0/16",
		"enable_dns": "true", "enable_vpc_peering": "true", "user_api_interface": "CQL",
		"alternator_write_isolation": "only_rmw_uses_lwt", "client_encryption_mode": "both", "replication_factor": "3",
		"availability_zone_ids.#": "1", "availability_zone_ids.0": "use1-az1",
		"encryption_at_rest.#": "1", "encryption_at_rest.0.enabled": "true", "encryption_at_rest.0.provider": "scylla-aws",
	}}
	cfg := map[string]interface{}{
		"name": "new-name", "cloud": "AWS", "region": "us-east-1", "client_encryption_mode": "encrypted",
		"node_type": "i8g.large", "min_nodes": 3, "cidr_block": "172.31.0.0/16",
	}
	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(cfg), nil)
	require.NoError(t, err)
	require.False(t, diff.RequiresNew())
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	require.NoError(t, err)
	require.True(t, d.HasChange("name"))
	require.True(t, d.HasChange("client_encryption_mode"))
	return d
}

func updateServer(t *testing.T, renameStatus string) *scyllatest.Server {
	t.Helper()
	srv := scyllatest.New(t)
	dc := map[string]any{"id": 5}
	srv.Reply(http.MethodGet, "/account/1/cluster/3", map[string]any{"cluster": map[string]any{"id": 3, "encryptionMode": "both", "dc": dc, "dataCenters": []any{dc}}})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dc/5", map[string]any{"dataCenter": dc})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})
	srv.Reply(http.MethodPatch, "/account/1/cluster/3/name", model.ClusterRequest{ID: 50, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/50", model.ClusterRequest{ID: 50, Status: renameStatus})
	srv.Reply(http.MethodPut, "/account/1/cluster/3/encryption/mode", model.ClusterRequest{ID: 60, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/60", model.ClusterRequest{ID: 60, Status: "COMPLETED"})
	return srv
}

func TestResourceClusterUpdateAppliesEveryChange(t *testing.T) {
	t.Parallel()
	srv := updateServer(t, "COMPLETED")

	diags := resourceClusterUpdate(context.Background(), updateData(t), srv.Client(t))
	require.False(t, diags.HasError(), "%v", diags)

	var order []string
	for _, c := range srv.Calls() {
		if c.Method != http.MethodGet {
			order = append(order, c.Method+" "+c.Path)
		}
	}
	require.Equal(t, []string{
		"PATCH /account/1/cluster/3/name",
		"PUT /account/1/cluster/3/encryption/mode",
	}, order)
}

func TestResourceClusterUpdateStopsAtFirstError(t *testing.T) {
	t.Parallel()
	srv := updateServer(t, "FAILED")

	diags := resourceClusterUpdate(context.Background(), updateData(t), srv.Client(t))
	require.True(t, diags.HasError())
	require.Empty(t, srv.CallsTo(http.MethodPut, "/account/1/cluster/3/encryption/mode"))
}

func TestBackupRetentionDaysDescribesDeleteOnly(t *testing.T) {
	t.Parallel()
	require.Contains(t, ResourceCluster().Schema["backup_retention_days"].Description, "only when the cluster is deleted")
}
