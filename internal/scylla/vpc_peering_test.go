package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestUpdateClusterVPCPeeringReadsBack(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/cluster/3/network/vpc/peer/8"
	// The PUT answers with Siren's PascalCase struct, which is ignored.
	srv.Reply(http.MethodPut, path, map[string]any{"ID": 8, "PeerVPCIPv4CIDRList": []any{"10.2.0.0/16"}})
	srv.Reply(http.MethodGet, path, map[string]any{"id": 8, "status": "ACTIVE", "cidrList": []any{"10.2.0.0/16", "10.3.0.0/16"}})

	p, err := srv.Client(t).UpdateClusterVPCPeering(context.Background(), 3, 8, &model.VPCPeeringUpdateRequest{
		Status: "ACTIVE", CIDRBlock: "10.2.0.0/16,10.3.0.0/16",
	})
	require.NoError(t, err)
	require.Equal(t, int64(8), p.ID)
	require.Equal(t, []string{"10.2.0.0/16", "10.3.0.0/16"}, p.CIDRList)
	require.Equal(t, map[string]any{"status": "ACTIVE", "cidrBlock": "10.2.0.0/16,10.3.0.0/16"}, srv.Body(t, http.MethodPut, path))
}
