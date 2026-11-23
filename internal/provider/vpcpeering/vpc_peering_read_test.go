package vpcpeering

import (
	"fmt"
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
// lists cluster 42 without a datacenter, or sends an error for a cluster it
// could not read. Reading cluster 42 fails, so the read must find the peering
// without reading it. It must not remove the peering from the state while one
// of these clusters may list it. A refresh looks in the cluster of the state
// first, and fails when only another cluster lists the peering.
func TestVPCPeeringReadClusterWithoutDatacenter(t *testing.T) {
	t.Parallel()

	const (
		// The API omits dcCount for a cluster without a datacenter.
		cluster42 = `{"id":42,"status":"QUEUED"}`
		// cluster43 and cluster44 have the peering, as clusters that share
		// a network do.
		cluster43 = `{"id":43,"status":"ACTIVE","dcCount":1}`
		cluster44 = `{"id":44,"status":"ACTIVE","dcCount":1}`
		// cluster45 has no peering.
		cluster45 = `{"id":45,"status":"ACTIVE","dcCount":1}`
		// unreadable is what the API sends for a cluster it could not read.
		unreadable = `{"error":{"code":"040023","message":"General Request Error","trace_id":"t1"}}`

		errStateClusterWithoutDC = `cluster 42 of VPC peering "pcx-1" is listed without a datacenter, so the peering cannot be read: ` +
			`try again when the cluster has an ACTIVE datacenter`
		errUnchecked = `VPC peering "pcx-1" was not found, but 1 of the clusters of the account could not be read, ` +
			`and they may list it: try again later`

		// stateRm and checkImport are parts of the error details.
		stateRm     = "terraform state rm"
		checkImport = "An import takes the first cluster that lists the peering."
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
		// wantDetail is a part of the detail of the error, "" for none.
		wantDetail string
	}{
		{
			name:          "import",
			clusters:      []string{cluster42, cluster43},
			wantClusterID: 43,
		},
		{
			name:          "import while the API could not read a cluster",
			clusters:      []string{unreadable, cluster43},
			wantClusterID: 43,
		},
		{
			name:     "import of a peering that no cluster lists",
			clusters: []string{cluster45},
		},
		{
			name:     "import while only a cluster without a datacenter could list the peering",
			clusters: []string{cluster42, cluster45},
			wantErr:  errUnchecked,
		},
		{
			name:     "import while only a cluster the API could not read could list the peering",
			clusters: []string{unreadable, cluster45},
			wantErr:  errUnchecked,
		},
		{
			name:           "refresh",
			stateClusterID: 43,
			clusters:       []string{cluster42, cluster43},
			wantClusterID:  43,
		},
		{
			name:           "refresh while the API could not read a cluster",
			stateClusterID: 43,
			clusters:       []string{unreadable, cluster43},
			wantClusterID:  43,
		},
		{
			name:           "refresh while the cluster of the state has no datacenter",
			stateClusterID: 42,
			clusters:       []string{cluster42},
			wantErr:        errStateClusterWithoutDC,
			wantDetail:     stateRm,
		},
		{
			// The clusters of a shared network list the same peering, but
			// taking it from another cluster would change cluster_id.
			name:           "refresh while the cluster of the state has no datacenter and another cluster lists the peering",
			stateClusterID: 42,
			clusters:       []string{cluster42, cluster43},
			wantErr:        errStateClusterWithoutDC,
			wantDetail:     stateRm,
		},
		{
			name:           "refresh of a peering that a cluster listed first also has",
			stateClusterID: 44,
			clusters:       []string{cluster43, cluster44},
			wantClusterID:  44,
		},
		{
			// The network outlives a deleted cluster while other clusters
			// use it, and so does the peering.
			name:           "refresh while the cluster of the state is gone and another cluster lists the peering",
			stateClusterID: 43,
			clusters:       []string{cluster44},
			wantErr:        `VPC peering "pcx-1" is not listed by cluster 43 of the state but by cluster 44: to manage it through that cluster, remove it from the state and import it again`,
			wantDetail:     checkImport,
		},
		{
			name:           "refresh while the cluster of the state lists no peering and another cluster lists it",
			stateClusterID: 45,
			clusters:       []string{cluster45, cluster44},
			wantErr:        `VPC peering "pcx-1" is not listed by cluster 45 of the state but by cluster 44: to manage it through that cluster, remove it from the state and import it again`,
			wantDetail:     checkImport,
		},
		{
			// The list has no ID for a cluster the API could not read, and
			// that cluster can be cluster 43.
			name:           "refresh while the cluster of the state is not listed, the API could not read a cluster and another cluster lists the peering",
			stateClusterID: 43,
			clusters:       []string{unreadable, cluster44},
			wantErr: `VPC peering "pcx-1" is listed by cluster 44, but cluster 43 of the state is not listed, ` +
				`and the API could not read 1 of the clusters of the account: try again later`,
			wantDetail: stateRm,
		},
		{
			name:           "refresh while the cluster of the state lists no peering, the API could not read a cluster and another cluster lists it",
			stateClusterID: 45,
			clusters:       []string{unreadable, cluster45, cluster44},
			wantErr:        `VPC peering "pcx-1" is not listed by cluster 45 of the state but by cluster 44: to manage it through that cluster, remove it from the state and import it again`,
			wantDetail:     checkImport,
		},
		{
			name:           "refresh while the cluster of the state is gone and no cluster lists the peering",
			stateClusterID: 43,
			clusters:       []string{cluster45},
		},
		{
			name:           "refresh while the cluster of the state is gone and only a cluster without a datacenter could list the peering",
			stateClusterID: 43,
			clusters:       []string{cluster42, cluster45},
			wantErr:        errUnchecked,
			wantDetail:     stateRm,
		},
		{
			name:           "refresh while the cluster of the state is gone and only a cluster the API could not read could list the peering",
			stateClusterID: 43,
			clusters:       []string{unreadable, cluster45},
			wantErr:        errUnchecked,
			wantDetail:     stateRm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const peering = `{"id":9,"externalId":"pcx-1","vpcId":"vpc-1","ownerId":"123",` +
				`"cidrList":["10.0.0.0/16"],"allowCql":true}`

			writeCluster := func(w http.ResponseWriter, id int, peerings string) {
				_, _ = fmt.Fprintf(w, `{"data":{"cluster":{"id":%d,"status":"ACTIVE","cloudProviderId":1,`+
					`"dc":{"id":%d,"name":"AWS_US_EAST_1"},"dataCenters":[{"id":%d,"name":"AWS_US_EAST_1"}],`+
					`"vpcPeeringList":[%s]}}}`, id, id, id, peerings)
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch req := r.Method + " " + r.URL.Path; req {
				case "GET /account/7/clusters":
					_, _ = w.Write([]byte(`{"data":{"clusters":[` + strings.Join(tt.clusters, ",") + `]}}`))
				case "GET /account/7/cluster/42":
					t.Errorf("read cluster 42, which has no datacenter")
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":42,"status":"QUEUED","cloudProviderId":1}}}`))
				case "GET /account/7/cluster/43":
					writeCluster(w, 43, peering)
				case "GET /account/7/cluster/44":
					writeCluster(w, 44, peering)
				case "GET /account/7/cluster/45":
					writeCluster(w, 45, "")
				case "GET /account/7/cluster/43/dc/43", "GET /account/7/cluster/44/dc/44", "GET /account/7/cluster/45/dc/45":
					_, _ = w.Write([]byte(`{"data":{"name":"AWS_US_EAST_1"}}`))
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
				if tt.wantDetail == "" {
					require.Empty(t, diags[0].Detail)
				} else {
					require.Contains(t, diags[0].Detail, tt.wantDetail)
				}
				require.Equal(t, "pcx-1", d.Id(), "the peering must stay in the state")
				require.Equal(t, tt.stateClusterID, d.Get("cluster_id"), "the peering must keep its cluster_id")
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
