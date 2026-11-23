package vpcpeering

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// TestVPCPeeringReadClusterWithoutDatacenter reads a peering while the API
// lists cluster 42 without a datacenter. Reading such a cluster fails, so the
// read must find the peering without reading it, and must not remove a
// peering of that cluster from the state.
func TestVPCPeeringReadClusterWithoutDatacenter(t *testing.T) {
	t.Parallel()

	const (
		// The API omits dcCount for a cluster without a datacenter.
		cluster42 = `{"id":42,"status":"QUEUED"}`
		// cluster43 has the peering.
		cluster43 = `{"id":43,"status":"ACTIVE","dcCount":1}`
	)

	tests := []struct {
		name string
		// stateClusterID is the cluster_id in the state, 0 on import.
		stateClusterID int
		clusters       []string
		// wantClusterID is the cluster of the peering read, 0 when the
		// read removes the peering from the state.
		wantClusterID int
		wantErr       string
	}{
		{
			name:          "import",
			clusters:      []string{cluster42, cluster43},
			wantClusterID: 43,
		},
		{
			name:     "import of a peering that is gone",
			clusters: []string{cluster42},
		},
		{
			name:           "refresh",
			stateClusterID: 43,
			clusters:       []string{cluster42, cluster43},
			wantClusterID:  43,
		},
		{
			name:           "refresh of a peering whose cluster is gone",
			stateClusterID: 43,
			clusters:       []string{cluster42},
		},
		{
			// The cluster of the state is listed without a datacenter, so
			// the read must look for the peering in the other clusters.
			name:           "refresh that finds the peering in another cluster",
			stateClusterID: 42,
			clusters:       []string{cluster42, cluster43},
			wantClusterID:  43,
		},
		{
			name:           "refresh while the cluster of the state has no datacenter",
			stateClusterID: 42,
			clusters:       []string{cluster42},
			wantErr:        `cluster 42 of VPC peering "pcx-1" is listed without a datacenter, so the peering cannot be read: try again later`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch req := r.Method + " " + r.URL.Path; req {
				case "GET /account/7/clusters":
					_, _ = w.Write([]byte(`{"data":{"clusters":[` + strings.Join(tt.clusters, ",") + `]}}`))
				case "GET /account/7/cluster/42":
					t.Errorf("read cluster 42, which has no datacenter")
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":42,"status":"QUEUED","cloudProviderId":1}}}`))
				case "GET /account/7/cluster/43":
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":43,"status":"ACTIVE","cloudProviderId":1,` +
						`"dc":{"id":5,"name":"AWS_US_EAST_1"},"dataCenters":[{"id":5,"name":"AWS_US_EAST_1"}],` +
						`"vpcPeeringList":[{"id":9,"externalId":"pcx-1","vpcId":"vpc-1","ownerId":"123",` +
						`"cidrList":["10.0.0.0/16"],"allowCql":true}]}}}`))
				case "GET /account/7/cluster/43/dc/5":
					_, _ = w.Write([]byte(`{"data":{"id":5,"name":"AWS_US_EAST_1"}}`))
				default:
					t.Errorf("unexpected request %s", req)
				}
			}))
			t.Cleanup(srv.Close)

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

			d := ResourceVPCPeering().TestResourceData()
			d.SetId("pcx-1")
			if tt.stateClusterID != 0 {
				require.NoError(t, d.Set("cluster_id", tt.stateClusterID))
			}

			diags := resourceVPCPeeringRead(t.Context(), d, client)

			if tt.wantErr != "" {
				require.True(t, diags.HasError())
				require.Equal(t, tt.wantErr, diags[0].Summary)
				require.Equal(t, "pcx-1", d.Id(), "the peering must stay in the state")
				return
			}
			require.False(t, diags.HasError(), "%v", diags)

			if tt.wantClusterID == 0 {
				require.Empty(t, d.Id(), "the peering must be removed from the state")
				return
			}

			require.Equal(t, "pcx-1", d.Id())
			require.Equal(t, tt.wantClusterID, d.Get("cluster_id"))
			require.Equal(t, "AWS_US_EAST_1", d.Get("datacenter"))
			require.Equal(t, 9, d.Get("vpc_peering_id"))
		})
	}
}
