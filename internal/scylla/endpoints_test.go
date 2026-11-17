package scylla

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"
)

// TestListClustersPlaceholderInstance checks a cluster that has no instance
// yet. The API returns a placeholder instance for it, with a numeric
// environment, and the list must still decode.
func TestListClustersPlaceholderInstance(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method+" "+r.URL.Path != "GET /account/7/clusters" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"clusters":[{"id":42,"status":"QUEUED","instance":` +
			`{"id":0,"externalId":"unknown instance","cloudProviderId":1,"costPerHour":0,"environment":0}}]}}`))
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
	require.NoError(t, err)
	require.Len(t, clusters, 1)
	require.Equal(t, int64(42), clusters[0].ID)
}
