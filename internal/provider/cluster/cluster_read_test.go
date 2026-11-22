package cluster

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// placeholderInstance is the instance the API sends for a cluster when it
// cannot look the instance up: while the datacenter has no ACTIVE node, so
// its instance ID is 0, or when the lookup of a valid ID fails.
const placeholderInstance = `{"id":0,"externalId":"unknown instance","cloudProviderId":1,` +
	`"memory":0,"localDiskCount":0,"totalStorage":0,"cpuCount":0,"networkSpeed":0,` +
	`"externalStorageNetworkSpeed":0,"costPerHour":0,"environment":0,` +
	`"licenseCostOnDemandPerHour":0,"subscriptionCostHourly":0,"instanceCostHourly":0}`

// TestResourceClusterReadInstance reads a Standard cluster for which the API
// cannot report the instance. A refresh must keep node_type, node_disk_size
// and min_nodes, so the next plan neither replaces nor resizes the cluster.
func TestResourceClusterReadInstance(t *testing.T) {
	t.Parallel()

	const (
		realInstance = `{"id":2,"externalId":"i3.large","totalStorage":475}`
		activeNodes  = `[{"id":1,"status":"ACTIVE"},{"id":2,"status":"ACTIVE"},{"id":3,"status":"ACTIVE"}]`
	)

	// managed is the state of a three-node cluster that Terraform created.
	managed := map[string]any{
		"name":           "cluster",
		"cloud":          "AWS",
		"region":         "us-east-1",
		"node_type":      "i3.large",
		"node_disk_size": 475,
		"min_nodes":      3,
	}

	tests := []struct {
		name string
		// state is the state before the read, nil for an import.
		state      map[string]any
		instanceID int
		instance   string
		nodes      string
		wantErr    string
		wantCount  int
	}{
		{
			name:       "refresh while no node is ACTIVE",
			state:      managed,
			instanceID: 0,
			instance:   placeholderInstance,
			nodes:      `[]`,
			wantCount:  0,
		},
		{
			name:       "import while no node is ACTIVE",
			instanceID: 0,
			instance:   placeholderInstance,
			nodes:      `[]`,
			wantErr:    "cluster 42 has no ACTIVE node, so its node_type is unknown: try again when it has ACTIVE nodes",
		},
		{
			name:       "refresh when the instance lookup of the API fails",
			state:      managed,
			instanceID: 2,
			instance:   placeholderInstance,
			nodes:      activeNodes,
			wantCount:  3,
		},
		{
			name:       "import",
			instanceID: 2,
			instance:   realInstance,
			nodes:      activeNodes,
			wantCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dc := fmt.Sprintf(`{"id":5,"name":"AWS_US_EAST_1","instanceId":%d}`, tt.instanceID)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /account/7/cluster/42/request":
					_, _ = w.Write([]byte(`{"data":[{"id":11,"status":"COMPLETED"}]}`))
				case "GET /account/7/cluster/42":
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":42,"clusterName":"cluster","status":"ACTIVE",` +
						`"cloudProviderId":1,"userApiInterface":"CQL","broadcastType":"PRIVATE","dns":true,` +
						`"region":{"id":1,"externalId":"us-east-1"},"scyllaVersion":{"version":"2025.1.0"},` +
						`"instance":` + tt.instance + `,"dc":` + dc + `,"dataCenters":[` + dc + `],` +
						`"nodes":` + tt.nodes + `}}}`))
				case "GET /account/7/cluster/42/dc/5":
					_, _ = w.Write([]byte(`{"data":` + dc + `}`))
				case "GET /deployment/cloud-provider/1/region/1":
					_, _ = w.Write([]byte(`{"data":{"instances":[` + realInstance + `]}}`))
				case "GET /account/7/cluster/42/certificate":
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":"041201"}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			defer srv.Close()

			endpoint, err := url.Parse(srv.URL)
			require.NoError(t, err)

			client := &scylla.Client{
				Endpoint:   endpoint,
				Headers:    make(http.Header),
				HTTPClient: srv.Client(),
				Retry:      retrier.New(nil, nil),
				AccountID:  7,
				Meta: &scylla.Cloudmeta{CloudProviders: []scylla.CloudProvider{{
					CloudProvider:        &model.CloudProvider{ID: 1, Name: "AWS"},
					CloudProviderRegions: &model.CloudProviderRegions{},
				}}},
			}

			d := ResourceCluster().TestResourceData()
			d.SetId("42")
			for k, v := range tt.state {
				require.NoError(t, d.Set(k, v))
			}

			diags := resourceClusterRead(t.Context(), d, client)
			if tt.wantErr != "" {
				require.True(t, diags.HasError())
				require.Equal(t, tt.wantErr, diags[0].Summary)
				return
			}
			require.False(t, diags.HasError(), "%v", diags)

			for k, v := range managed {
				require.Equal(t, v, d.Get(k), k)
			}
			require.Equal(t, tt.wantCount, d.Get("node_count"))

			// The configuration of the managed cluster plans no change to the
			// values that resize or replace it.
			config := map[string]cty.Value{
				"name":           cty.StringVal("cluster"),
				"cloud":          cty.StringVal("AWS"),
				"region":         cty.StringVal("us-east-1"),
				"node_type":      cty.StringVal("i3.large"),
				"node_disk_size": cty.NumberIntVal(475),
				"min_nodes":      cty.NumberIntVal(3),
			}
			diff, err := clusterDiff(t, d.State(), config)
			require.NoError(t, err)
			if diff == nil {
				return
			}
			for _, k := range []string{"node_type", "node_disk_size", "min_nodes"} {
				// The SDK can list an attribute whose value does not change.
				if a := diff.Attributes[k]; a != nil {
					require.Equal(t, a.Old, a.New, k)
					require.False(t, a.RequiresNew || a.NewComputed, k)
				}
			}
		})
	}
}
