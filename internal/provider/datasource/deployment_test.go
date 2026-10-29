package datasource

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func deploymentServer(t *testing.T) *scyllatest.Server {
	t.Helper()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/deployment/cloud-providers", map[string]any{"cloudProviders": []any{
		map[string]any{"id": 1, "name": "AWS"}, map[string]any{"id": 2, "name": "GCP"},
	}})
	srv.Reply(http.MethodGet, "/deployment/cloud-provider/1/regions", map[string]any{"regions": []any{
		map[string]any{"id": 1, "externalId": "us-east-1", "fullName": "US East (N. Virginia)", "continent": "na", "dcName": "AWS_US_EAST_1"},
	}})
	return srv
}

func raw(t *testing.T, r *schema.Resource, m map[string]any) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, r.Schema, m)
}

func TestDataSourceCloudProviders(t *testing.T) {
	t.Parallel()
	srv := deploymentServer(t)
	d := DataSourceCloudProviders().TestResourceData()
	require.False(t, dataSourceCloudProvidersRead(context.Background(), d, srv.Client(t)).HasError())
	require.Len(t, d.Get("cloud_providers"), 2)
}

func TestDataSourceRegions(t *testing.T) {
	t.Parallel()
	srv := deploymentServer(t)
	d := raw(t, DataSourceRegions(), map[string]any{"cloud": "aws"})
	require.False(t, dataSourceRegionsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{
		"region_id": 1, "name": "us-east-1", "full_name": "US East (N. Virginia)", "continent": "na", "datacenter": "AWS_US_EAST_1",
	}}, d.Get("regions"))

	d = raw(t, DataSourceRegions(), map[string]any{"cloud": "AZURE"})
	diags := dataSourceRegionsRead(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, `unknown cloud "AZURE"`)
}

func TestDataSourceInstanceTypes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		target, query string
	}{{"CLUSTER", ""}, {"VECTOR_SEARCH", "target=VECTOR_SEARCH"}} {
		srv := deploymentServer(t)
		srv.Reply(http.MethodGet, "/deployment/cloud-provider/1/region/1", map[string]any{"instances": []any{
			map[string]any{"id": 182, "externalId": "i8g.large", "instanceFamily": "i8g", "cpuCount": 2, "memory": 16384, "totalStorage": 468},
		}})
		d := raw(t, DataSourceInstanceTypes(), map[string]any{"region": "us-east-1", "target": tc.target})

		require.False(t, dataSourceInstanceTypesRead(context.Background(), d, srv.Client(t)).HasError(), tc.target)
		require.Equal(t, []any{map[string]any{
			"instance_type_id": 182, "name": "i8g.large", "family": "i8g", "cpu_count": 2, "memory": 16384, "total_storage": 468,
		}}, d.Get("instance_types"))
		calls := srv.CallsTo(http.MethodGet, "/deployment/cloud-provider/1/region/1")
		require.Len(t, calls, 1)
		require.Equal(t, tc.query, calls[0].Query, tc.target)
	}
}

func TestDataSourceInstanceTypesUnknownRegion(t *testing.T) {
	t.Parallel()
	srv := deploymentServer(t)
	d := raw(t, DataSourceInstanceTypes(), map[string]any{"region": "mars-1"})
	diags := dataSourceInstanceTypesRead(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, `unknown AWS region "mars-1"`)
}

func TestDataSourceScyllaVersions(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/deployment/scylla-versions", map[string]any{"defaultScyllaVersionId": 330, "scyllaVersions": []any{
		map[string]any{"id": 329, "version": "2024.2.1", "description": "old"},
		map[string]any{"id": 330, "version": "2025.1.2", "description": "new"},
	}})
	d := DataSourceScyllaVersions().TestResourceData()
	require.False(t, dataSourceScyllaVersionsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "2025.1.2", d.Get("default_version"))
	require.Len(t, d.Get("versions"), 2)
}

func TestDataSourceAvailabilityZonesDefaultsToScyllaAccount(t *testing.T) {
	t.Parallel()
	srv := deploymentServer(t)
	srv.Reply(http.MethodGet, "/account/1/cloud-account", []any{
		map[string]any{"id": 9, "cloudProviderId": 1, "owner": "Customer", "state": "ACTIVE"},
		map[string]any{"id": 4, "cloudProviderId": 1, "owner": "Scylla", "state": "ACTIVE"},
	})
	srv.Reply(http.MethodGet, "/account/1/cloud-account/4/region/1/zones", []any{
		map[string]any{"id": "use1-az2", "name": "us-east-1b"}, map[string]any{"id": "use1-az1", "name": "us-east-1a"},
	})
	d := raw(t, DataSourceAvailabilityZones(), map[string]any{"region": "us-east-1"})

	require.False(t, dataSourceAvailabilityZonesRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, 4, d.Get("cloud_account_id"))
	require.Equal(t, []any{"use1-az1", "use1-az2"}, d.Get("zone_ids"))
}

func TestDataSourceClusterRequests(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []any{
		map[string]any{"id": 50, "requestType": "CREATE_CLUSTER", "status": "COMPLETED", "progressPercent": 100, "progressDescription": "ready"},
	})
	d := raw(t, DataSourceClusterRequests(), map[string]any{"cluster_id": 3, "status": "COMPLETED"})

	require.False(t, dataSourceClusterRequestsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{map[string]any{
		"request_id": 50, "type": "CREATE_CLUSTER", "status": "COMPLETED", "progress_percent": 100, "progress_description": "ready",
	}}, d.Get("requests"))
	require.Equal(t, "status=COMPLETED", srv.CallsTo(http.MethodGet, "/account/1/cluster/3/request")[0].Query)
}
