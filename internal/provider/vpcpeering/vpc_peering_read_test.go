package vpcpeering

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// TestVPCPeeringReadSkipsClusterWithoutDatacenter lists a cluster that has no
// datacenter yet before the cluster that has the peering. Reading such a
// cluster fails, so the read must find the peering without reading it.
func TestVPCPeeringReadSkipsClusterWithoutDatacenter(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		requests []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := r.Method + " " + r.URL.Path

		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()

		switch req {
		case "GET /account/7/clusters":
			// The API omits dcCount for a cluster without a datacenter.
			_, _ = w.Write([]byte(`{"data":{"clusters":[{"id":42,"status":"QUEUED"},` +
				`{"id":43,"status":"ACTIVE","dcCount":1}]}}`))
		case "GET /account/7/cluster/42":
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

	d := ResourceVPCPeering().TestResourceData()
	d.SetId("pcx-1")

	diags := resourceVPCPeeringRead(t.Context(), d, client)
	require.False(t, diags.HasError(), "%v", diags)

	require.Equal(t, "pcx-1", d.Id())
	require.Equal(t, 43, d.Get("cluster_id"))
	require.Equal(t, "AWS_US_EAST_1", d.Get("datacenter"))
	require.Equal(t, 9, d.Get("vpc_peering_id"))

	mu.Lock()
	defer mu.Unlock()
	require.False(t, slices.Contains(requests, "GET /account/7/cluster/42"), "read the cluster without a datacenter: %v", requests)
}
