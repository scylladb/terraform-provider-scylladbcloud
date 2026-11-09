package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const vectorInstancesPath = "/deployment/cloud-provider/1/region/2"

func vectorInstances() model.CloudProviderInstances {
	return model.CloudProviderInstances{Instances: []model.CloudProviderInstance{
		{ID: 7, ExternalID: "r7g.large"},
		{ID: 8, ExternalID: "r7g.xlarge"},
	}}
}

func TestClusterVectorSearchSchema(t *testing.T) {
	t.Parallel()
	s := ResourceCluster().Schema["vector_search"]
	require.True(t, s.Optional)
	require.False(t, s.ForceNew, "a later change must not replace the cluster")
	require.False(t, s.Computed)
	require.Equal(t, 1, s.MaxItems)
}

func TestClusterCreateRequestVectorSearchJSON(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(model.ClusterCreateRequest{})
	require.NoError(t, err)
	require.NotContains(t, string(b), "vectorSearch", "requests without the block must not change")

	b, err = json.Marshal(model.ClusterCreateRequest{VectorSearch: &model.VectorSearchBootstrap{NodeCount: 2, DefaultInstanceTypeID: 7}})
	require.NoError(t, err)
	require.Contains(t, string(b), `"vectorSearch":{"nodeCount":2,"defaultInstanceTypeId":7}`)
}

func TestVectorSearchBootstrapResolvesNodeType(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, vectorInstancesPath, vectorInstances())

	got, err := vectorSearchBootstrap(context.Background(), srv.Client(t), 1, 2,
		map[string]interface{}{"node_type": "R7G.XLARGE", "node_count": 3})
	require.NoError(t, err)
	require.Equal(t, &model.VectorSearchBootstrap{NodeCount: 3, DefaultInstanceTypeID: 8}, got)

	calls := srv.CallsTo(http.MethodGet, vectorInstancesPath)
	require.Len(t, calls, 1)
	require.Equal(t, "target=VECTOR_SEARCH", calls[0].Query)
}

func TestVectorSearchBootstrapUnknownNodeType(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, vectorInstancesPath, vectorInstances())

	_, err := vectorSearchBootstrap(context.Background(), srv.Client(t), 1, 2,
		map[string]interface{}{"node_type": "m5.large", "node_count": 1})
	require.ErrorContains(t, err, `node_type "m5.large" is not available in this region; available: r7g.large, r7g.xlarge`)
}

func TestAwaitVectorSearchInstall(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request",
		[]model.ClusterRequest{{ID: 60, RequestType: installVectorSearchRequestType, Status: "QUEUED"}})
	srv.Sequence(http.MethodGet, "/account/1/cluster/request/60",
		scyllatest.Response{Data: model.ClusterRequest{ID: 60, Status: "QUEUED"}},
		scyllatest.Response{Data: model.ClusterRequest{ID: 60, Status: "IN_PROGRESS"}},
		scyllatest.Response{Data: model.ClusterRequest{ID: 60, Status: "COMPLETED"}},
	)

	require.NoError(t, awaitVectorSearchInstall(context.Background(), srv.Client(t), 3))

	calls := srv.CallsTo(http.MethodGet, "/account/1/cluster/3/request")
	require.Len(t, calls, 1)
	require.Equal(t, "type=INSTALL_VECTOR_SEARCH", calls[0].Query)
	require.Len(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/request/60"), 3)
}

func TestAwaitVectorSearchInstallFails(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request",
		[]model.ClusterRequest{{ID: 60, RequestType: installVectorSearchRequestType, Status: "QUEUED"}})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/60", model.ClusterRequest{ID: 60, Status: "FAILED"})

	require.ErrorContains(t, awaitVectorSearchInstall(context.Background(), srv.Client(t), 3), "request 60")
}

func TestAwaitVectorSearchInstallMissing(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})

	require.ErrorContains(t, awaitVectorSearchInstall(context.Background(), srv.Client(t), 3), "no INSTALL_VECTOR_SEARCH request")
}

func TestVectorSearchChangeError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		exists, changed, set bool
		wantErr              bool
	}{
		{"set at creation", false, true, true, false},
		{"unchanged", true, false, true, false},
		{"added or changed later", true, true, true, true},
		{"removed later", true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := vectorSearchChangeError(tc.exists, tc.changed, tc.set)
			if tc.wantErr {
				require.ErrorIs(t, err, errVectorSearchChanged)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// vectorSearchPlan plans an existing cluster whose state and configuration
// carry the given vector_search blocks (nil for none).
func vectorSearchPlan(t *testing.T, inState, inConfig map[string]interface{}) error {
	t.Helper()
	attrs := map[string]string{
		"id": "3", "cluster_id": "3", "name": "c", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": "3", "cidr_block": "172.31.0.0/16",
		"enable_dns": "true", "enable_vpc_peering": "true", "user_api_interface": "CQL",
		"alternator_write_isolation": "only_rmw_uses_lwt",
		"availability_zone_ids.#":    "1", "availability_zone_ids.0": "use1-az1",
		"encryption_at_rest.#": "1", "encryption_at_rest.0.enabled": "true", "encryption_at_rest.0.provider": "scylla-aws",
	}
	if inState != nil {
		attrs["vector_search.#"] = "1"
		attrs["vector_search.0.node_count"] = inState["node_count"].(string)
		attrs["vector_search.0.node_type"] = inState["node_type"].(string)
	}
	state := &terraform.InstanceState{ID: "3", Attributes: attrs, Meta: map[string]interface{}{"schema_version": "3"}}
	cfg := map[string]interface{}{
		"name": "c", "cloud": "AWS", "region": "us-east-1",
		"node_type": "i8g.large", "min_nodes": 3, "cidr_block": "172.31.0.0/16",
	}
	if inConfig != nil {
		cfg["vector_search"] = []interface{}{inConfig}
	}
	_, err := ResourceCluster().Diff(context.Background(), state, terraform.NewResourceConfigRaw(cfg), nil)
	return err
}

func TestClusterPlanVectorSearch(t *testing.T) {
	t.Parallel()
	inState := map[string]interface{}{"node_count": "2", "node_type": "r7g.large"}

	require.NoError(t, vectorSearchPlan(t, inState, map[string]interface{}{"node_count": 2, "node_type": "r7g.large"}), "unchanged")
	require.NoError(t, vectorSearchPlan(t, inState, nil), "removed")
	require.NoError(t, vectorSearchPlan(t, nil, nil), "never set")
	require.ErrorIs(t, vectorSearchPlan(t, nil, map[string]interface{}{"node_count": 2, "node_type": "r7g.large"}), errVectorSearchChanged, "added")
	require.ErrorIs(t, vectorSearchPlan(t, inState, map[string]interface{}{"node_count": 4, "node_type": "r7g.large"}), errVectorSearchChanged, "resized")
}
