package acctest

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestSweepClusters(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: []model.Cluster{
		{ID: 1, ClusterName: "tf-acc-test-basic-aws-x62svfkq", Status: "ACTIVE"},
		{ID: 2, ClusterName: "production", Status: "ACTIVE"},
		{ID: 3, ClusterName: "tf-acc-test-gone-a1b2c3d4", Status: "DELETED"},
		{ID: 4, ClusterName: "tf-acc-test-basic-gcp-q9w8e7r6", Status: "BOOTSTRAP_ERROR"},
		{ID: 5, ClusterName: "basic-gcp-7877416753363943680", Status: "BOOTSTRAP_ERROR"},
	}})
	for _, id := range []int{1, 4} {
		testNoNetwork(srv, id)
	}
	srv.Reply(http.MethodPost, "/account/1/cluster/1/delete", model.ClusterRequest{ID: 10})
	srv.Fail(http.MethodPost, "/account/1/cluster/4/delete", http.StatusBadRequest, "040001")

	var logs []string
	logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }

	err := SweepClusters(context.Background(), srv.Client(t), logf)

	// A failed delete does not stop the sweep, and its error is returned.
	require.ErrorContains(t, err, `cluster 4 "tf-acc-test-basic-gcp-q9w8e7r6"`)
	require.Len(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/1/delete"), 1)
	require.Len(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/4/delete"), 1)
	// Clusters without the prefix, and deleted clusters, are never touched.
	for _, id := range []int{2, 3, 5} {
		require.Empty(t, srv.CallsTo(http.MethodPost, fmt.Sprintf("/account/1/cluster/%d/delete", id)))
	}
	require.Contains(t, logs, `[INFO] sweeper: deleted cluster 1 "tf-acc-test-basic-aws-x62svfkq" (status ACTIVE)`)
	require.Contains(t, logs, "[INFO] sweeper: deleted 1 clusters, 1 failed")
}

// testNoNetwork makes the fake API answer that cluster id has no VPC
// peerings and no connections.
func testNoNetwork(srv *scyllatest.Server, id int) {
	srv.Reply(http.MethodGet, fmt.Sprintf("/account/1/cluster/%d/network/vpc/peer", id), []model.VPCPeering{})
	srv.Reply(http.MethodGet, fmt.Sprintf("/account/1/cluster/%d/network/vpc/connection", id), map[string]any{"connections": []model.ClusterConnection{}})
}

// A cluster delete leaves its VPC peerings, and after it they cannot be
// listed. So the sweeper deletes the peerings and the connections first,
// and a failed peering delete does not stop the cluster delete.
func TestSweepClustersDeletesNetworkFirst(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: []model.Cluster{
		{ID: 1, ClusterName: "tf-acc-test-basic-aws-x62svfkq", Status: "ACTIVE"},
	}})
	srv.Reply(http.MethodGet, "/account/1/cluster/1/network/vpc/peer", []model.VPCPeering{
		{ID: 7, ExternalID: "pcx-7"},
		{ID: 8, ExternalID: "pcx-8"},
	})
	srv.Reply(http.MethodGet, "/account/1/cluster/1/network/vpc/connection", map[string]any{
		"connections": []model.ClusterConnection{{ID: 9}},
	})
	srv.Reply(http.MethodDelete, "/account/1/cluster/1/network/vpc/peer/7", nil)
	srv.Fail(http.MethodDelete, "/account/1/cluster/1/network/vpc/peer/8", http.StatusInternalServerError, "000000")
	srv.Reply(http.MethodDelete, "/account/1/cluster/1/network/vpc/connection/9", nil)
	srv.Reply(http.MethodPost, "/account/1/cluster/1/delete", model.ClusterRequest{ID: 10})

	var logs []string
	logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }

	err := SweepClusters(context.Background(), srv.Client(t), logf)

	require.ErrorContains(t, err, "cannot delete VPC peering 8")
	var deletes []string
	for _, c := range srv.Calls() {
		if c.Method != http.MethodGet {
			deletes = append(deletes, c.Method+" "+c.Path)
		}
	}
	require.Equal(t, []string{
		"DELETE /account/1/cluster/1/network/vpc/peer/7",
		"DELETE /account/1/cluster/1/network/vpc/peer/8",
		"DELETE /account/1/cluster/1/network/vpc/connection/9",
		"POST /account/1/cluster/1/delete",
	}, deletes)
	require.Contains(t, logs, "[INFO] sweeper: deleted VPC peering 7 (pcx-7) of cluster 1")
	require.Contains(t, logs, "[INFO] sweeper: deleted connection 9 of cluster 1")
	require.Contains(t, logs, "[INFO] sweeper: deleted 1 clusters, 1 failed")
}

func TestSweepClustersListError(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, "/account/1/clusters", http.StatusUnauthorized, "Unauthorized")

	err := SweepClusters(context.Background(), srv.Client(t), func(string, ...any) {})

	require.ErrorContains(t, err, "failed to list clusters")
}
