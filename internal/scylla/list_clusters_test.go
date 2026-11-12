package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

// A cluster whose creation failed comes back with an empty instance holding 0
// in string fields; it must not break listing the other clusters.
func TestListClustersWithFailedCluster(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", map[string]any{"clusters": []any{
		map[string]any{"id": 93, "status": "ACTIVE", "instance": map[string]any{"environment": "PRODUCTION"}},
		map[string]any{"id": 94, "status": "BOOTSTRAP_ERROR", "instance": map[string]any{"environment": 0}},
		map[string]any{"id": 95, "status": "ACTIVE", "instance": map[string]any{"environment": nil}},
	}})

	clusters, err := srv.Client(t).ListClusters(context.Background())
	require.NoError(t, err)
	require.Len(t, clusters, 3)
	require.EqualValues(t, "PRODUCTION", clusters[0].Instance.Environment)
	require.EqualValues(t, "0", clusters[1].Instance.Environment)
	require.EqualValues(t, "", clusters[2].Instance.Environment)
}
