package cluster

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestClientEncryptionModeSchema(t *testing.T) {
	t.Parallel()
	s := ResourceCluster().Schema["client_encryption_mode"]
	require.True(t, s.Optional)
	require.True(t, s.Computed)
	require.False(t, s.ForceNew)

	for _, v := range []string{"encrypted", "both"} {
		require.False(t, s.ValidateDiagFunc(v, cty.Path{}).HasError(), v)
	}
	for _, v := range []string{"unencrypted", "BOTH", ""} {
		require.True(t, s.ValidateDiagFunc(v, cty.Path{}).HasError(), v)
	}
}

// encryptionModeServer serves cluster 3 in mode, accepts a mode change and
// reports the new mode on the next read.
func encryptionModeServer(t *testing.T, mode string) *scyllatest.Server {
	t.Helper()
	srv := scyllatest.New(t)
	dc := map[string]any{"id": 5}
	cluster := func(m string) scyllatest.Response {
		return scyllatest.Response{Data: map[string]any{"cluster": map[string]any{"id": 3, "encryptionMode": m, "dc": dc, "dataCenters": []any{dc}}}}
	}
	srv.Sequence(http.MethodGet, "/account/1/cluster/3", cluster(mode), cluster("encrypted"))
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dc/5", map[string]any{"dataCenter": dc})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{})
	srv.Reply(http.MethodPut, "/account/1/cluster/3/encryption/mode", model.ClusterRequest{ID: 60, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/60", model.ClusterRequest{ID: 60, Status: "COMPLETED"})
	return srv
}

func TestEnsureClientEncryptionModeChanges(t *testing.T) {
	t.Parallel()
	srv := encryptionModeServer(t, "both")
	c := srv.Client(t)
	cluster, err := c.GetCluster(context.Background(), 3)
	require.NoError(t, err)

	got, err := ensureClientEncryptionMode(context.Background(), c, cluster, "encrypted")
	require.NoError(t, err)
	require.Equal(t, "encrypted", got.EncryptionMode)

	calls := srv.CallsTo(http.MethodPut, "/account/1/cluster/3/encryption/mode")
	require.Len(t, calls, 1)
	var body map[string]string
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]string{"mode": "encrypted"}, body)
}

func TestEnsureClientEncryptionModeSkipsCurrentMode(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"both", ""} {
		srv := encryptionModeServer(t, "both")
		c := srv.Client(t)
		cluster, err := c.GetCluster(context.Background(), 3)
		require.NoError(t, err)

		got, err := ensureClientEncryptionMode(context.Background(), c, cluster, want)
		require.NoError(t, err)
		require.Same(t, cluster, got)
		require.Empty(t, srv.CallsTo(http.MethodPut, "/account/1/cluster/3/encryption/mode"), "want %q", want)
	}
}

func TestEnsureClientEncryptionModeFailedRequest(t *testing.T) {
	t.Parallel()
	srv := encryptionModeServer(t, "both")
	srv.Reply(http.MethodGet, "/account/1/cluster/request/60", model.ClusterRequest{ID: 60, Status: "FAILED"})
	c := srv.Client(t)
	cluster, err := c.GetCluster(context.Background(), 3)
	require.NoError(t, err)

	_, err = ensureClientEncryptionMode(context.Background(), c, cluster, "encrypted")
	require.ErrorContains(t, err, "cluster request ID 60 failed")
}

func TestSetClusterKVsSetsClientEncryptionMode(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"both", "encrypted", "unencrypted", ""} {
		d := ResourceCluster().TestResourceData()
		cluster := &model.Cluster{
			ID:             3,
			EncryptionMode: mode,
			Status:         "ACTIVE",
			Region:         &model.CloudProviderRegion{ExternalID: "us-east-1"},
			ScyllaVersion:  &model.ScyllaVersion{Version: "2025.1"},
			Datacenter:     &model.Datacenter{Name: "AWS_US_EAST_1", CIDRBlock: "172.31.0.0/16"},
			Datacenters:    []model.Datacenter{{}},
		}
		require.NoError(t, setClusterKVs(d, cluster, "AWS", "", "", nil, &scylla.CloudProvider{}))
		require.Equal(t, mode, d.Get("client_encryption_mode"))
	}
}
