package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestClusterDestroyed(t *testing.T) {
	t.Parallel()
	const path = "/account/1/cluster/3"

	tests := []struct {
		name    string
		reply   func(*scyllatest.Server)
		wantErr string
	}{
		{name: "deleted", reply: func(s *scyllatest.Server) { s.Fail(http.MethodGet, path, http.StatusOK, "040001") }},
		{name: "cluster deleted message", reply: func(s *scyllatest.Server) { s.Fail(http.MethodGet, path, http.StatusOK, "CLUSTER_DELETED") }},
		{name: "not found", reply: func(s *scyllatest.Server) { s.Fail(http.MethodGet, path, http.StatusNotFound, "NOT_FOUND") }},
		{
			name: "still exists",
			reply: func(s *scyllatest.Server) {
				s.Reply(http.MethodGet, path, map[string]any{"cluster": map[string]any{"id": 3}})
			},
			wantErr: "cluster 3 still exists",
		},
		{
			name:    "unauthorized",
			reply:   func(s *scyllatest.Server) { s.Fail(http.MethodGet, path, http.StatusOK, "020006") },
			wantErr: "checking that cluster 3 was destroyed",
		},
		{
			name:    "server error",
			reply:   func(s *scyllatest.Server) { s.Fail(http.MethodGet, path, http.StatusInternalServerError, "000000") },
			wantErr: "checking that cluster 3 was destroyed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			tt.reply(srv)

			err := clusterDestroyed(context.Background(), srv.Client(t), 3)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
