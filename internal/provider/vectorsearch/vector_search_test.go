package vectorsearch

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const vsPath = "/account/1/cluster/3/dc/5/vector-search"

func server(t *testing.T) *scyllatest.Server {
	t.Helper()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dcs", map[string]any{"dataCenters": []any{
		map[string]any{"id": 5, "Name": "AWS_US_EAST_1", "CloudProviderID": 1, "regionID": 1},
	}})
	srv.Reply(http.MethodGet, "/deployment/cloud-provider/1/region/1", map[string]any{"instances": []any{
		map[string]any{"id": 176, "externalId": "t4g.medium"},
		map[string]any{"id": 204, "externalId": "r7g.large"},
	}})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})
	return srv
}

func installed(nodes int, instanceType int) map[string]any {
	ns := []any{}
	for i := 0; i < nodes; i++ {
		ns = append(ns, map[string]any{"id": 10 + i, "instanceTypeId": instanceType, "status": "ACTIVE", "up": true})
	}
	return map[string]any{"status": "HEALTHY", "availabilityZones": []any{
		map[string]any{"azid": "use1-az1", "rackName": "RACK0", "nodes": ns},
		map[string]any{"azid": "use1-az2", "rackName": "RACK1"},
	}}
}

func data(t *testing.T, raw map[string]any) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, ResourceVectorSearch().Schema, raw)
}

func TestParseID(t *testing.T) {
	t.Parallel()
	c, dc, err := parseID("3:5")
	require.NoError(t, err)
	require.Equal(t, [2]int64{3, 5}, [2]int64{c, dc})
	for _, bad := range []string{"3", "3:x", "a:5", "3:5:1", ""} {
		_, _, err := parseID(bad)
		require.Error(t, err, bad)
	}
}

func TestCreate(t *testing.T) {
	t.Parallel()
	srv := server(t)
	srv.Reply(http.MethodPost, vsPath, model.ClusterRequest{ID: 70, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/70", model.ClusterRequest{ID: 70, Status: "COMPLETED"})
	srv.Reply(http.MethodGet, vsPath, installed(2, 204))
	d := data(t, map[string]any{"cluster_id": 3, "datacenter": "aws_us_east_1", "node_count": 2, "node_type": "r7g.large"})

	diags := resourceVectorSearchCreate(context.Background(), d, srv.Client(t))
	require.False(t, diags.HasError(), "%v", diags)
	require.Equal(t, "3:5", d.Id())
	require.Equal(t, "HEALTHY", d.Get("status"))
	require.Equal(t, []any{"use1-az1"}, d.Get("availability_zones"))
	require.Equal(t, 2, d.Get("node_count"))
	require.Equal(t, "r7g.large", d.Get("node_type"))
	require.Equal(t, "AWS_US_EAST_1", d.Get("datacenter"))

	calls := srv.CallsTo(http.MethodPost, vsPath)
	require.Len(t, calls, 1)
	var body map[string]int
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]int{"nodeCount": 2, "defaultInstanceTypeId": 204}, body)
}

func TestCreateUnknownNodeType(t *testing.T) {
	t.Parallel()
	srv := server(t)
	d := data(t, map[string]any{"cluster_id": 3, "datacenter": "AWS_US_EAST_1", "node_count": 2, "node_type": "x9.huge"})

	diags := resourceVectorSearchCreate(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, "t4g.medium, r7g.large")
	require.Empty(t, srv.CallsTo(http.MethodPost, vsPath))
}

func TestCreateUnknownDatacenter(t *testing.T) {
	t.Parallel()
	srv := server(t)
	d := data(t, map[string]any{"cluster_id": 3, "datacenter": "GCP_X", "node_count": 2, "node_type": "r7g.large"})

	diags := resourceVectorSearchCreate(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, `no datacenter "GCP_X"`)
}

func TestReadNotInstalledRemovesState(t *testing.T) {
	t.Parallel()
	srv := server(t)
	srv.Reply(http.MethodGet, vsPath, map[string]any{"status": "UNKNOWN", "availabilityZones": []any{}})
	d := data(t, map[string]any{"cluster_id": 3, "datacenter": "AWS_US_EAST_1", "node_count": 2, "node_type": "r7g.large"})
	d.SetId("3:5")

	require.False(t, resourceVectorSearchRead(context.Background(), d, srv.Client(t)).HasError())
	require.Empty(t, d.Id())
}

func TestUpdateResizes(t *testing.T) {
	t.Parallel()
	srv := server(t)
	srv.Reply(http.MethodPatch, vsPath, model.ClusterRequest{ID: 71, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/71", model.ClusterRequest{ID: 71, Status: "COMPLETED"})
	srv.Reply(http.MethodGet, vsPath, installed(4, 176))
	d := data(t, map[string]any{"cluster_id": 3, "datacenter": "AWS_US_EAST_1", "node_count": 4, "node_type": "t4g.medium"})
	d.SetId("3:5")

	require.False(t, resourceVectorSearchUpdate(context.Background(), d, srv.Client(t)).HasError())
	calls := srv.CallsTo(http.MethodPatch, vsPath)
	require.Len(t, calls, 1)
	var body map[string]int
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]int{"nodeCount": 4, "defaultInstanceTypeId": 176}, body)
	require.Equal(t, 4, d.Get("node_count"))
	require.Equal(t, "t4g.medium", d.Get("node_type"))
}

func TestDelete(t *testing.T) {
	t.Parallel()
	srv := server(t)
	srv.Reply(http.MethodDelete, vsPath, model.ClusterRequest{ID: 72, Status: "QUEUED"})
	srv.Sequence(http.MethodGet, "/account/1/cluster/request/72",
		scyllatest.Response{Data: model.ClusterRequest{ID: 72, Status: "IN_PROGRESS"}},
		scyllatest.Response{Data: model.ClusterRequest{ID: 72, Status: "COMPLETED"}},
	)
	d := data(t, map[string]any{"cluster_id": 3, "datacenter": "AWS_US_EAST_1", "node_count": 2, "node_type": "r7g.large"})
	d.SetId("3:5")

	require.False(t, resourceVectorSearchDelete(context.Background(), d, srv.Client(t)).HasError())
	require.Len(t, srv.CallsTo(http.MethodDelete, vsPath), 1)
	require.Len(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/request/72"), 2)
}

func TestImport(t *testing.T) {
	t.Parallel()
	d := ResourceVectorSearch().TestResourceData()
	d.SetId("3:5")
	out, err := resourceVectorSearchImport(context.Background(), d, nil)
	require.NoError(t, err)
	require.Equal(t, 3, out[0].Get("cluster_id"))
	require.Equal(t, 5, out[0].Get("datacenter_id"))
}
