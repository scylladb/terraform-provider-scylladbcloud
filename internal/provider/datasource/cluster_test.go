package datasource

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func withCluster(t *testing.T, r *schema.Resource) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, r.Schema, map[string]any{"cluster_id": 3})
}

func TestDataSourceClustersSkipsDeleted(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", map[string]any{"clusters": []any{
		map[string]any{"id": 3, "clusterName": "a", "status": "ACTIVE", "region": map[string]any{"externalId": "us-east-1"}, "cloudProvider": map[string]any{"name": "AWS"}},
		map[string]any{"id": 4, "clusterName": "b", "status": "DELETED"},
	}})
	d := DataSourceClusters().TestResourceData()

	require.False(t, dataSourceClustersRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{"cluster_id": 3, "name": "a", "status": "ACTIVE", "cloud": "AWS", "region": "us-east-1"}}, d.Get("clusters"))
}

func TestDataSourceCluster(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	dc := map[string]any{"id": 5, "Name": "AWS_US_EAST_1"}
	srv.Reply(http.MethodGet, "/account/1/cluster/3", map[string]any{"cluster": map[string]any{
		"id": 3, "clusterName": "a", "status": "ACTIVE", "replicationFactor": 3, "encryptionMode": "both",
		"userApiInterface": "CQL", "dns": true, "dc": dc, "dataCenters": []any{dc},
		"scyllaVersion": map[string]any{"version": "2025.1.2"},
		"nodes": []any{
			map[string]any{"id": 1, "status": "ACTIVE", "privateIP": "10.0.0.1", "dns": "n1"},
			map[string]any{"id": 2, "status": "DELETED", "privateIP": "10.0.0.2", "dns": "n2"},
		},
	}})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dc/5", map[string]any{"dataCenter": dc})
	d := withCluster(t, DataSourceCluster())

	require.False(t, dataSourceClusterRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "a", d.Get("name"))
	require.Equal(t, 1, d.Get("node_count"))
	require.Equal(t, []any{"10.0.0.1"}, d.Get("node_private_ips"))
	require.Equal(t, []any{"n1"}, d.Get("node_dns_names"))
	require.Equal(t, 3, d.Get("replication_factor"))
	require.Equal(t, "both", d.Get("client_encryption_mode"))
	require.Equal(t, "AWS_US_EAST_1", d.Get("datacenter"))
	require.Equal(t, "2025.1.2", d.Get("scylla_version"))
}

func TestDataSourceClusterNodes(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/nodes", map[string]any{"nodes": []any{
		map[string]any{"id": 1, "status": "ACTIVE", "state": "NORMAL", "privateIP": "10.0.0.1", "publicIP": "", "dns": "n1", "dcID": 5, "instanceId": 182},
	}})
	d := withCluster(t, DataSourceClusterNodes())

	require.False(t, dataSourceClusterNodesRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{
		"node_id": 1, "status": "ACTIVE", "state": "NORMAL", "private_ip": "10.0.0.1", "public_ip": "",
		"dns": "n1", "datacenter_id": 5, "instance_id": 182,
	}}, d.Get("nodes"))
}

func TestDataSourceClusterDatacenters(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dcs", map[string]any{"dataCenters": []any{
		map[string]any{"id": 5, "Name": "AWS_US_EAST_1", "Status": "ACTIVE", "CloudProviderID": 1, "regionID": 1, "cidrBlock": "172.31.0.0/16", "ReplicationFactor": 3},
	}})
	d := withCluster(t, DataSourceClusterDatacenters())

	require.False(t, dataSourceClusterDatacentersRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{
		"datacenter_id": 5, "name": "AWS_US_EAST_1", "status": "ACTIVE", "cloud_provider_id": 1,
		"region_id": 1, "cidr_block": "172.31.0.0/16", "replication_factor": 3,
	}}, d.Get("datacenters"))
}
