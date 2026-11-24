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

func TestSweepClustersListError(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, "/account/1/clusters", http.StatusUnauthorized, "Unauthorized")

	err := SweepClusters(context.Background(), srv.Client(t), func(string, ...any) {})

	require.ErrorContains(t, err, "failed to list clusters")
}
