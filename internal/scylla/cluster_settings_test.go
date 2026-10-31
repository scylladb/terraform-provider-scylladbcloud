package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestClusterSettingsEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		path     string
		call     func(*scylla.Client) (*model.ClusterRequest, error)
		wantBody map[string]any
	}{
		{
			name:   "rename",
			method: http.MethodPatch,
			path:   "/account/1/cluster/3/name",
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.RenameCluster(context.Background(), 3, "new-name")
			},
			wantBody: map[string]any{"name": "new-name"},
		},
		{
			name:   "client encryption mode",
			method: http.MethodPut,
			path:   "/account/1/cluster/3/encryption/mode",
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.SetClientEncryptionMode(context.Background(), 3, model.ClientEncryptionModeBoth)
			},
			wantBody: map[string]any{"mode": "both"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Reply(tt.method, tt.path, scyllatest.ClusterRequest)

			r, err := tt.call(srv.Client(t))
			require.NoError(t, err)
			require.Equal(t, int64(42), r.ID)
			require.Equal(t, int64(3), r.ClusterID)
			require.Equal(t, "QUEUED", r.Status)
			require.Equal(t, tt.wantBody, srv.Body(t, tt.method, tt.path))
		})
	}
}

func TestClusterEncryptionModeDecodes(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	dc := map[string]any{"id": 5}
	srv.Reply(http.MethodGet, "/account/1/cluster/3", map[string]any{"cluster": map[string]any{"id": 3, "encryptionMode": "both", "dc": dc, "dataCenters": []any{dc}}})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dc/5", map[string]any{"dataCenter": dc})

	c, err := srv.Client(t).GetCluster(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, "both", c.EncryptionMode)
}
