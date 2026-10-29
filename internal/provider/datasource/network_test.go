package datasource

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func replyCluster(srv *scyllatest.Server, broadcast string) {
	srv.Reply(http.MethodGet, "/account/1/cluster/3", map[string]any{"cluster": map[string]any{"id": 3, "broadcastType": broadcast}})
}

func TestDataSourceVPCPeerings(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	replyCluster(srv, "PRIVATE")
	srv.Reply(http.MethodGet, "/account/1/cluster/3/network/vpc/peer", []any{
		map[string]any{"id": 8, "externalId": "pcx-1", "vpcId": "vpc-1", "ownerId": "123", "regionId": 1, "cidrList": []any{"10.2.0.0/16"}, "status": "ACTIVE", "allowCql": true},
	})
	d := withCluster(t, DataSourceVPCPeerings())

	require.False(t, dataSourceVPCPeeringsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{
		"vpc_peering_id": 8, "connection_id": "pcx-1", "peer_vpc_id": "vpc-1", "peer_account_id": "123",
		"peer_region_id": 1, "peer_cidr_blocks": []any{"10.2.0.0/16"}, "status": "ACTIVE", "allow_cql": true,
	}}, d.Get("vpc_peerings"))
}

func TestDataSourceClusterConnections(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	replyCluster(srv, "PRIVATE")
	srv.Reply(http.MethodGet, "/account/1/cluster/3/network/vpc/connection", map[string]any{"connections": []any{
		map[string]any{
			"id": 9, "name": "tgw", "type": "AWS_TGW_ATTACHMENT", "status": "ACTIVE", "externalId": "tgw-attach-1",
			"clusterDCID": 5, "cidrList": []any{"10.1.0.0/16"}, "data": map[string]any{"tgwid": "tgw-1"},
		},
	}})
	d := withCluster(t, DataSourceClusterConnections())

	require.False(t, dataSourceClusterConnectionsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{
		"connection_id": 9, "name": "tgw", "type": "AWS_TGW_ATTACHMENT", "status": "ACTIVE", "external_id": "tgw-attach-1",
		"datacenter_id": 5, "cidrlist": []any{"10.1.0.0/16"}, "data": map[string]any{"tgwid": "tgw-1"},
	}}, d.Get("connections"))
}

func TestDataSourceCloudAccounts(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cloud-account", []any{
		map[string]any{"id": 1, "cloudProviderId": 1, "owner": "Scylla", "state": "ACTIVE"},
	})
	d := DataSourceCloudAccounts().TestResourceData()

	require.False(t, dataSourceCloudAccountsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{"cloud_account_id": 1, "cloud_provider_id": 1, "owner": "Scylla", "state": "ACTIVE"}}, d.Get("cloud_accounts"))
}

func TestDataSourcePublicClusterHasNoPeeringsOrConnections(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	replyCluster(srv, "PUBLIC")

	peerings := withCluster(t, DataSourceVPCPeerings())
	require.False(t, dataSourceVPCPeeringsRead(context.Background(), peerings, srv.Client(t)).HasError())
	require.Empty(t, peerings.Get("vpc_peerings"))

	conns := withCluster(t, DataSourceClusterConnections())
	require.False(t, dataSourceClusterConnectionsRead(context.Background(), conns, srv.Client(t)).HasError())
	require.Empty(t, conns.Get("connections"))

	require.Len(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/3"), 2)
	require.Len(t, srv.Calls(), 2)
}

func TestDataSourceListErrors(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	replyCluster(srv, "PRIVATE")
	srv.Fail(http.MethodGet, "/account/1/cluster/3/network/vpc/peer", http.StatusOK, "040001")
	require.True(t, dataSourceVPCPeeringsRead(context.Background(), withCluster(t, DataSourceVPCPeerings()), srv.Client(t)).HasError())

	missing := scyllatest.New(t)
	missing.Fail(http.MethodGet, "/account/1/cluster/3", http.StatusOK, "040001")
	require.True(t, dataSourceClusterConnectionsRead(context.Background(), withCluster(t, DataSourceClusterConnections()), missing.Client(t)).HasError())
}
