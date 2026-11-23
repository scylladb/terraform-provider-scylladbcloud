package vpcpeering

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

// TestVPCPeeringDeleteDeletedCluster deletes peering pcx-1 of cluster 43. The
// API answers 040001 when cluster 43 was deleted, but the network outlives a
// deleted cluster while other clusters use it, and so does the peering. The
// delete succeeds, which removes the peering from the state, only when a
// complete search finds no cluster that lists it.
func TestVPCPeeringDeleteDeletedCluster(t *testing.T) {
	t.Parallel()

	const (
		// The API omits dcCount for a cluster without a datacenter.
		cluster42 = `{"id":42,"status":"QUEUED"}`
		// cluster44 has the peering, cluster45 has none.
		cluster44 = `{"id":44,"status":"ACTIVE","dcCount":1}`
		cluster45 = `{"id":45,"status":"ACTIVE","dcCount":1}`
		// unreadable is what the API sends for a cluster it could not read.
		unreadable = `{"error":{"code":"040023","message":"General Request Error","trace_id":"t1"}}`

		errUnchecked = `cluster 43 of VPC peering "pcx-1" was deleted, but 1 of the clusters of the account could not be read, ` +
			`and they may list the peering: try again later`
	)

	tests := []struct {
		name string
		// deleteErr is the error code of the delete, "" when it succeeds.
		deleteErr string
		clusters  []string
		// wantList is whether the delete lists the clusters.
		wantList bool
		wantErr  string
		// wantDetail is a part of the detail of the error, "" for none.
		wantDetail string
	}{
		{
			name: "delete",
		},
		{
			name:      "the delete fails",
			deleteErr: "041130",
			wantErr:   "error deleting vpc peering",
		},
		{
			name:      "cluster deleted and no cluster lists the peering",
			deleteErr: "040001",
			clusters:  []string{cluster45},
			wantList:  true,
		},
		{
			name:      "cluster deleted and another cluster lists the peering",
			deleteErr: "040001",
			clusters:  []string{cluster45, cluster44},
			wantList:  true,
			wantErr: `cluster 43 of VPC peering "pcx-1" was deleted, but cluster 44 still lists the peering: ` +
				`to delete it through that cluster, remove it from the state, import it again and destroy it`,
			wantDetail: "An import takes the first cluster that lists the peering.",
		},
		{
			name:       "cluster deleted and a cluster without a datacenter could list the peering",
			deleteErr:  "040001",
			clusters:   []string{cluster42, cluster45},
			wantList:   true,
			wantErr:    errUnchecked,
			wantDetail: "terraform state rm",
		},
		{
			name:       "cluster deleted and a cluster the API could not read could list the peering",
			deleteErr:  "040001",
			clusters:   []string{unreadable, cluster45},
			wantList:   true,
			wantErr:    errUnchecked,
			wantDetail: "terraform state rm",
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

			var listed atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch req := r.Method + " " + r.URL.Path; req {
				case "DELETE /account/7/cluster/43/network/vpc/peer/9":
					switch tt.deleteErr {
					case "":
						_, _ = w.Write([]byte(`{"data":null}`))
					case "040001":
						// What the API answers for a deleted cluster.
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"error":"040001"}`))
					default:
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"error":"` + tt.deleteErr + `"}`))
					}
				case "GET /account/7/clusters":
					listed.Store(true)
					_, _ = w.Write([]byte(`{"data":{"clusters":[` + strings.Join(tt.clusters, ",") + `]}}`))
				case "GET /account/7/cluster/44":
					writeCluster(w, 44, peering)
				case "GET /account/7/cluster/45":
					writeCluster(w, 45, "")
				case "GET /account/7/cluster/44/dc/44", "GET /account/7/cluster/45/dc/45":
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
			}

			d := ResourceVPCPeering().TestResourceData()
			d.SetId("pcx-1")
			require.NoError(t, d.Set("cluster_id", 43))
			require.NoError(t, d.Set("vpc_peering_id", 9))

			diags := resourceVPCPeeringDelete(t.Context(), d, client)

			require.Equal(t, tt.wantList, listed.Load(), "the delete lists the clusters")

			if tt.wantErr == "" {
				require.Empty(t, diags)
				return
			}

			require.True(t, diags.HasError())
			require.Contains(t, diags[0].Summary, tt.wantErr)
			if tt.wantDetail == "" {
				require.Empty(t, diags[0].Detail)
			} else {
				require.Contains(t, diags[0].Detail, tt.wantDetail)
			}
		})
	}
}
