package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestSweepClusters(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", map[string]any{"clusters": []any{
		map[string]any{"id": 1, "clusterName": "tf-acc-test-basic-aws-1", "status": "ACTIVE"},
		map[string]any{"id": 2, "clusterName": "tf-dev", "status": "ACTIVE"},
		map[string]any{"id": 3, "clusterName": "tf-acc-test-gone-3", "status": "DELETED"},
		map[string]any{"id": 4, "clusterName": "tf-acc-test-ear-4", "status": "ACTIVE"},
	}})
	srv.Reply(http.MethodPost, "/account/1/cluster/1/delete", model.ClusterRequest{ID: 10})
	srv.Fail(http.MethodPost, "/account/1/cluster/4/delete", http.StatusOK, "040001")

	err := sweepClusters(context.Background(), srv.Client(t))
	require.ErrorContains(t, err, `cluster 4 "tf-acc-test-ear-4"`)
	require.NotContains(t, err.Error(), "tf-dev")

	require.Len(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/1/delete"), 1)
	require.Len(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/4/delete"), 1)
	require.Empty(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/2/delete"))
	require.Empty(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/3/delete"))
}
