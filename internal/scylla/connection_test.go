package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestRecreateClusterConnection(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/cluster/3/network/vpc/connection/9"
	srv.Reply(http.MethodPut, path, scyllatest.ClusterRequest)

	r, err := srv.Client(t).RecreateClusterConnection(context.Background(), 3, 9, &model.ClusterConnectionRecreateRequest{
		Name: "tgw", CIDRList: []string{"10.1.0.0/16"}, Data: map[string]string{"tgwid": "tgw-1"}, Status: "ACTIVE",
	})
	require.NoError(t, err)
	require.Equal(t, int64(42), r.ID)
	require.Equal(t, map[string]any{"name": "tgw", "cidrList": []any{"10.1.0.0/16"}, "data": map[string]any{"tgwid": "tgw-1"}, "status": "ACTIVE"}, srv.Body(t, http.MethodPut, path))
}
