package scyllatest_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestClientDoesNotRetry(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, "/account/1/cluster/3", http.StatusServiceUnavailable, "")

	_, err := srv.Client(t).GetClusterSummary(context.Background(), 3)
	require.Error(t, err)
	require.Len(t, srv.Calls(), 1)
}
