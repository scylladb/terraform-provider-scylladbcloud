package acctest

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func testLogs() (*[]string, func(string, ...any)) {
	var logs []string
	return &logs, func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }
}

// testDelete makes the fake API accept the delete of cluster id with request
// reqID, and answer the request with status.
func testDelete(srv *scyllatest.Server, id, reqID int64, status string) {
	srv.Reply(http.MethodPost, fmt.Sprintf("/account/1/cluster/%d/delete", id), model.ClusterRequest{ID: reqID, Status: "QUEUED"})
	srv.Reply(http.MethodGet, fmt.Sprintf("/account/1/cluster/request/%d", reqID), model.ClusterRequest{ID: reqID, Status: status})
}

func TestSweepClusters(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: []model.Cluster{
		{ID: 1, ClusterName: "tf-acc-test-basic-aws-x62svfkq", Status: "ACTIVE", BroadcastType: "PUBLIC"},
		{ID: 2, ClusterName: "production", Status: "ACTIVE"},
		{ID: 3, ClusterName: "tf-acc-test-gone-a1b2c3d4", Status: "DELETED"},
		{ID: 4, ClusterName: "tf-acc-test-basic-gcp-q9w8e7r6", Status: "BOOTSTRAP_ERROR", BroadcastType: "PUBLIC"},
		{ID: 5, ClusterName: "basic-gcp-7877416753363943680", Status: "BOOTSTRAP_ERROR"},
		{ID: 6, ClusterName: "tf-acc-test-ear-aws-k2j3h4g5", Status: "ACTIVE", BroadcastType: "PUBLIC"},
	}})
	testDelete(srv, 1, 10, "COMPLETED")
	srv.Fail(http.MethodPost, "/account/1/cluster/4/delete", http.StatusBadRequest, "040001")
	testDelete(srv, 6, 11, "FAILED")
	logs, logf := testLogs()

	err := SweepClusters(context.Background(), srv.Client(t), logf)

	// A failed delete does not stop the sweep, and its error is returned.
	require.ErrorContains(t, err, `cluster 4 "tf-acc-test-basic-gcp-q9w8e7r6": cannot delete the cluster`)
	// A cluster is deleted only when its delete request completes.
	require.ErrorContains(t, err, `cluster 6 "tf-acc-test-ear-aws-k2j3h4g5": the delete did not complete`)
	// Clusters without the prefix, and deleted clusters, are never touched.
	for _, id := range []int{2, 3, 5} {
		require.Empty(t, srv.CallsTo(http.MethodPost, fmt.Sprintf("/account/1/cluster/%d/delete", id)))
	}
	// A public cluster has no VPC peerings or connections, and the API
	// rejects the calls to list them.
	require.Empty(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/1/network/vpc/peer"))
	require.Contains(t, *logs, `[INFO] sweeper: deleted cluster 1 "tf-acc-test-basic-aws-x62svfkq" (status ACTIVE)`)
	require.Contains(t, *logs, "[INFO] sweeper: deleted 1 clusters, 2 failed")
}

// A cluster delete leaves its VPC peerings, and after it they cannot be
// listed. So the sweeper deletes the peerings and the connections first and
// waits until they are gone.
func TestSweepClustersDeletesNetworkFirst(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: []model.Cluster{
		{ID: 1, ClusterName: "tf-acc-test-basic-aws-x62svfkq", Status: "ACTIVE", BroadcastType: "PRIVATE"},
	}})
	srv.Reply(http.MethodGet, "/account/1/cluster/1/network/vpc/peer", []model.VPCPeering{{ID: 7, ExternalID: "pcx-7"}})
	srv.Reply(http.MethodGet, "/account/1/cluster/1/network/vpc/connection", map[string]any{
		"connections": []model.ClusterConnection{{ID: 9}},
	})
	srv.Reply(http.MethodDelete, "/account/1/cluster/1/network/vpc/peer/7", nil)
	srv.Reply(http.MethodDelete, "/account/1/cluster/1/network/vpc/connection/9", nil)
	// The peering delete is an open cluster request until it completes.
	srv.Reply(http.MethodGet, "/account/1/cluster/1/request", []model.ClusterRequest{{ID: 20, Status: "IN_PROGRESS"}})
	srv.Sequence(http.MethodGet, "/account/1/cluster/request/20",
		scyllatest.Response{Data: model.ClusterRequest{ID: 20, Status: "IN_PROGRESS"}},
		scyllatest.Response{Data: model.ClusterRequest{ID: 20, Status: "COMPLETED"}},
	)
	srv.Sequence(http.MethodGet, "/account/1/cluster/1/network/vpc/connection/9",
		scyllatest.Response{Data: model.ClusterConnection{ID: 9, Status: "DELETING"}},
		scyllatest.Response{Status: http.StatusNotFound, Error: "Not Found"},
	)
	testDelete(srv, 1, 10, "COMPLETED")
	logs, logf := testLogs()

	err := SweepClusters(context.Background(), srv.Client(t), logf)

	require.NoError(t, err)
	var calls []string
	for _, c := range srv.Calls() {
		calls = append(calls, c.Method+" "+c.Path)
	}
	// Each call comes after the one before it in this list.
	last := -1
	for _, want := range []string{
		"DELETE /account/1/cluster/1/network/vpc/peer/7",
		"DELETE /account/1/cluster/1/network/vpc/connection/9",
		"GET /account/1/cluster/request/20",
		"GET /account/1/cluster/1/network/vpc/connection/9",
		"POST /account/1/cluster/1/delete",
	} {
		i := slices.Index(calls[last+1:], want)
		require.GreaterOrEqual(t, i, 0, "%q is missing or out of order in %q", want, calls)
		last += 1 + i
	}
	require.Contains(t, *logs, "[INFO] sweeper: deleting VPC peering 7 (pcx-7) of cluster 1")
	require.Contains(t, *logs, "[INFO] sweeper: deleted 1 clusters, 0 failed")
}

// After a failed peering delete, the cluster must stay. Otherwise the peering
// can no longer be listed, and no later sweep can find it.
func TestSweepClustersKeepsClusterAfterNetworkError(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: []model.Cluster{
		{ID: 1, ClusterName: "tf-acc-test-basic-aws-x62svfkq", Status: "ACTIVE", BroadcastType: "PRIVATE"},
	}})
	srv.Reply(http.MethodGet, "/account/1/cluster/1/network/vpc/peer", []model.VPCPeering{{ID: 8, ExternalID: "pcx-8"}})
	srv.Reply(http.MethodGet, "/account/1/cluster/1/network/vpc/connection", map[string]any{"connections": []model.ClusterConnection{}})
	srv.Fail(http.MethodDelete, "/account/1/cluster/1/network/vpc/peer/8", http.StatusInternalServerError, "000000")
	logs, logf := testLogs()

	err := SweepClusters(context.Background(), srv.Client(t), logf)

	require.ErrorContains(t, err, "cannot delete VPC peering 8")
	require.ErrorContains(t, err, "the cluster is kept, run the sweep again")
	require.Empty(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/1/delete"))
	require.Contains(t, *logs, "[INFO] sweeper: deleted 0 clusters, 1 failed")
}

func TestSweepClustersListError(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, "/account/1/clusters", http.StatusUnauthorized, "Unauthorized")

	err := SweepClusters(context.Background(), srv.Client(t), func(string, ...any) {})

	require.ErrorContains(t, err, "failed to list clusters")
}
