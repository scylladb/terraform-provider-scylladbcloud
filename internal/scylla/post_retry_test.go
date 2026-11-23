package scylla_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

// The API may run a cluster create that fails with a gateway error or a
// timeout. A second create then fails with "Cluster name is already used",
// and the first cluster runs outside Terraform.
func TestCreateClusterRetry(t *testing.T) {
	t.Parallel()
	ok := scyllatest.Response{Data: map[string]any{"requestId": 7}}
	for _, tt := range []struct {
		name        string
		first       scyllatest.Response
		slow        bool // the first answer comes after the client timeout
		wantPosts   int
		wantUnknown bool
	}{
		{name: "bad gateway", first: scyllatest.Response{Status: http.StatusBadGateway}, wantPosts: 1, wantUnknown: true},
		{name: "service unavailable", first: scyllatest.Response{Status: http.StatusServiceUnavailable}, wantPosts: 1, wantUnknown: true},
		{name: "gateway timeout", first: scyllatest.Response{Status: http.StatusGatewayTimeout}, wantPosts: 1, wantUnknown: true},
		{name: "request timeout", first: scyllatest.Response{Status: http.StatusRequestTimeout}, wantPosts: 1, wantUnknown: true},
		{name: "client timeout", first: ok, slow: true, wantPosts: 1, wantUnknown: true},
		{name: "too many requests is sent again", first: scyllatest.Response{Status: http.StatusTooManyRequests}, wantPosts: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			var n atomic.Int32
			srv.Handle(http.MethodPost, "/account/1/cluster", func(*http.Request, []byte) scyllatest.Response {
				if n.Add(1) > 1 {
					return ok
				}
				if tt.slow {
					time.Sleep(200 * time.Millisecond)
				}
				return tt.first
			})
			srv.Reply(http.MethodGet, "/account/1/cluster/request/7", model.ClusterRequest{ID: 7, ClusterID: 3})

			c := srv.Client(t)
			c.RetryOnce = retrier.New(retrier.ConstantBackoff(3, time.Millisecond), scylla.OnceClassifier)
			c.HTTPClient.Timeout = 100 * time.Millisecond

			_, err := c.CreateCluster(context.Background(), &model.ClusterCreateRequest{ClusterName: "test"})

			require.Len(t, srv.CallsTo(http.MethodPost, "/account/1/cluster"), tt.wantPosts)
			require.Equal(t, tt.wantUnknown, scylla.MayHaveRun(err), "error: %v", err)
			if !tt.wantUnknown {
				require.NoError(t, err)
			}
		})
	}
}

// No POST is sent again after an answer that shows the API may have run it,
// also when the client retries other requests.
func TestPostIsNotSentAgain(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		path string
		call func(context.Context, *scylla.Client) error
	}{
		{"DeleteCluster", "/account/1/cluster/3/delete", func(ctx context.Context, c *scylla.Client) error {
			_, err := c.DeleteCluster(ctx, 3, "test", 0)
			return err
		}},
		{"ResizeCluster", "/account/1/cluster/3/resize", func(ctx context.Context, c *scylla.Client) error {
			_, err := c.ResizeCluster(ctx, 3, 4, 5, 6)
			return err
		}},
		{"CreateAllowlistRule", "/account/1/cluster/3/network/firewall/allowed", func(ctx context.Context, c *scylla.Client) error {
			_, err := c.CreateAllowlistRule(ctx, 3, "192.0.2.1/32")
			return err
		}},
		{"CreateClusterVPCPeering", "/account/1/cluster/3/network/vpc/peer", func(ctx context.Context, c *scylla.Client) error {
			_, err := c.CreateClusterVPCPeering(ctx, 3, &model.VPCPeeringRequest{})
			return err
		}},
		{"CreateClusterConnection", "/account/1/cluster/3/network/vpc/connection", func(ctx context.Context, c *scylla.Client) error {
			_, err := c.CreateClusterConnection(ctx, 3, &model.ClusterConnectionCreateRequest{})
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Handle(http.MethodPost, tt.path, func(*http.Request, []byte) scyllatest.Response {
				return scyllatest.Response{Status: http.StatusGatewayTimeout}
			})

			c := srv.Client(t)
			backoff := retrier.ConstantBackoff(3, time.Millisecond)
			c.Retry = retrier.New(backoff, scylla.DefaultClassifier)
			c.RetryOnce = retrier.New(backoff, scylla.OnceClassifier)

			err := tt.call(context.Background(), c)

			require.Error(t, err)
			require.Len(t, srv.CallsTo(http.MethodPost, tt.path), 1)
		})
	}
}
