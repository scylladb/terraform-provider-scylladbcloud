package connection

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

// TestFindConnectionWithoutClusterID searches the clusters of the account for
// a connection, as an import does. The API fails the lookup in a public
// cluster, and a cluster without a datacenter can be public.
func TestFindConnectionWithoutClusterID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// clusters is the cluster list, before cluster 43 with the connection.
		clusters string
		// lookupStatus answers the lookup in cluster 43 when not 200.
		lookupStatus int
		wantErr      string
	}{
		{
			name:     "a cluster without a datacenter comes first",
			clusters: `{"id":42,"status":"QUEUED"},`,
		},
		{
			name:         "the lookup fails",
			lookupStatus: http.StatusInternalServerError,
			wantErr:      "error reading cluster connection 77",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				mu       sync.Mutex
				requests []string
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.URL.Path)
				mu.Unlock()

				switch r.Method + " " + r.URL.Path {
				case "GET /account/7/clusters":
					// The API omits dcCount for a cluster without a datacenter.
					_, _ = w.Write([]byte(`{"data":{"clusters":[` + tt.clusters + `{"id":43,"dcCount":1}]}}`))
				case "GET /account/7/cluster/42/network/vpc/connection/77":
					// What the API answers for a public cluster.
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"000000"}`))
				case "GET /account/7/cluster/43/network/vpc/connection/77":
					if tt.lookupStatus != 0 {
						w.WriteHeader(tt.lookupStatus)
						_, _ = w.Write([]byte(`{"error":"040023"}`))
						return
					}
					_, _ = w.Write([]byte(`{"data":{"id":77,"name":"conn"}}`))
				case "GET /account/7/cluster/43":
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":43,"dc":{"id":5},"dataCenters":[{"id":5}]}}}`))
				case "GET /account/7/cluster/43/dc/5":
					_, _ = w.Write([]byte(`{"data":{"id":5}}`))
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
			}

			cluster, connection, err := findConnection(t.Context(), client, 0, 77)

			mu.Lock()
			defer mu.Unlock()
			for _, path := range requests {
				require.False(t, strings.HasPrefix(path, "/account/7/cluster/42/"), "read cluster 42: %v", requests)
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, cluster)
				require.Nil(t, connection)
				return
			}

			require.NoError(t, err)
			require.Equal(t, int64(43), cluster.ID)
			require.Equal(t, int64(77), connection.ID)
		})
	}
}
