package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestGetClusterSummarySkipsDatacenterLookup(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3", map[string]any{"cluster": map[string]any{
		"id": 3, "dataCenters": []any{map[string]any{"id": 5}, map[string]any{"id": 6}},
	}})

	c, err := srv.Client(t).GetClusterSummary(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, int64(3), c.ID)
	require.Len(t, c.Datacenters, 2)
	require.Len(t, srv.Calls(), 1)
}

func TestListVectorSearchInstances(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/deployment/cloud-provider/1/region/2"
	srv.Reply(http.MethodGet, path, map[string]any{"instances": []any{map[string]any{"id": 77, "externalId": "r7g.large"}}})

	got, err := srv.Client(t).ListVectorSearchInstances(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(77), got[0].ID)
	calls := srv.CallsTo(http.MethodGet, path)
	require.Len(t, calls, 1)
	require.Equal(t, "target=VECTOR_SEARCH", calls[0].Query)
}
