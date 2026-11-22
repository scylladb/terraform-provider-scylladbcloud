package scylla

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// placeholderInstance is the instance the API sends for a cluster when it
// cannot look the instance up, for example while the cluster has no
// datacenter. All its fields are numbers, environment too.
const placeholderInstance = `{"id":0,"externalId":"unknown instance","cloudProviderId":1,` +
	`"memory":0,"localDiskCount":0,"totalStorage":0,"cpuCount":0,"networkSpeed":0,` +
	`"externalStorageNetworkSpeed":0,"costPerHour":0,"environment":0,` +
	`"licenseCostOnDemandPerHour":0,"subscriptionCostHourly":0,"instanceCostHourly":0}`

func TestListClusters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		clusters string
		want     []model.ClusterSummary
		wantErr  string
	}{
		{
			// The API omits dcCount for a cluster without a datacenter.
			name: "clusters with a placeholder instance",
			clusters: `[{"id":42,"status":"QUEUED","dataCenters":null,"instance":` + placeholderInstance + `},` +
				`{"id":43,"status":"ACTIVE","dcCount":1,"dataCenters":[{"id":5}],"instance":` + placeholderInstance + `}]`,
			want: []model.ClusterSummary{{ID: 42}, {ID: 43, DCCount: 1}},
		},
		{
			name: "a cluster the API could not read",
			clusters: `[{"id":43,"dcCount":1},` +
				`{"error":{"code":"040023","message":"General Request Error","trace_id":"t1"}}]`,
			wantErr: `the API could not read a cluster of the account: ` +
				`{"code":"040023","message":"General Request Error","trace_id":"t1"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method+" "+r.URL.Path != "GET /account/7/clusters" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					return
				}
				if got := r.URL.Query().Get("enriched"); got != "true" {
					t.Errorf(`enriched = %q, want "true"`, got)
				}
				_, _ = w.Write([]byte(`{"data":{"clusters":` + tt.clusters + `}}`))
			}))
			defer srv.Close()

			endpoint, err := url.Parse(srv.URL)
			require.NoError(t, err)

			c := &Client{
				Endpoint:   endpoint,
				Headers:    make(http.Header),
				HTTPClient: srv.Client(),
				Retry:      retrier.New(nil, nil),
				AccountID:  7,
			}

			clusters, err := c.ListClusters(t.Context())
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				require.Nil(t, clusters)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, clusters)
		})
	}
}
