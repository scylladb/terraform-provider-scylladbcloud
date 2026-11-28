package serverless

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

// A delete is not sent again after an unclear error, but the API can still
// run it. The next destroy then gets "cluster deleted", and must succeed.
func TestServerlessDeleteOfDeletedCluster(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodPost, "/account/1/cluster/3/delete", http.StatusBadRequest, "040001")
	d := ResourceServerlessCluster().TestResourceData()
	d.SetId("3")
	require.NoError(t, d.Set("name", "test"))

	diags := resourceServerlessClusterDelete(context.Background(), d, srv.Client(t))

	require.False(t, diags.HasError(), "%v", diags)
	require.Len(t, srv.CallsTo(http.MethodPost, "/account/1/cluster/3/delete"), 1)
}
