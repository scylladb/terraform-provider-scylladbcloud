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

func TestVectorSearchRequestEndpoints(t *testing.T) {
	t.Parallel()

	const path = "/account/1/cluster/3/dc/5/vector-search"
	tests := []struct {
		name     string
		method   string
		call     func(*scylla.Client) (*model.ClusterRequest, error)
		wantBody map[string]any
	}{
		{
			name:   "create",
			method: http.MethodPost,
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.CreateVectorSearch(context.Background(), 3, 5, &model.VectorSearchRequest{NodeCount: 2, DefaultInstanceTypeID: 77})
			},
			wantBody: map[string]any{"nodeCount": float64(2), "defaultInstanceTypeId": float64(77)},
		},
		{
			name:   "update",
			method: http.MethodPatch,
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.UpdateVectorSearch(context.Background(), 3, 5, &model.VectorSearchRequest{NodeCount: 4, DefaultInstanceTypeID: 77})
			},
			wantBody: map[string]any{"nodeCount": float64(4), "defaultInstanceTypeId": float64(77)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Reply(tt.method, path, scyllatest.ClusterRequest)

			r, err := tt.call(srv.Client(t))
			require.NoError(t, err)
			require.Equal(t, int64(42), r.ID)
			require.Equal(t, tt.wantBody, srv.Body(t, tt.method, path))
		})
	}
}

func TestDeleteVectorSearch(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/cluster/3/dc/5/vector-search"
	srv.Reply(http.MethodDelete, path, scyllatest.ClusterRequest)

	r, err := srv.Client(t).DeleteVectorSearch(context.Background(), 3, 5)
	require.NoError(t, err)
	require.Equal(t, int64(42), r.ID)
	require.Len(t, srv.CallsTo(http.MethodDelete, path), 1)
}

func TestGetVectorSearch(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/dc/5/vector-search", map[string]any{
		"status": "HEALTHY",
		"availabilityZones": []any{map[string]any{
			"azid": "use1-az1", "rackName": "RACK0",
			"nodes": []any{map[string]any{"id": 11, "instanceTypeId": 77, "status": "ACTIVE", "up": true}},
		}},
	})

	vs, err := srv.Client(t).GetVectorSearch(context.Background(), 3, 5)
	require.NoError(t, err)
	require.Equal(t, &model.VectorSearch{
		Status: model.VectorSearchStatusHealthy,
		AvailabilityZones: []model.VectorSearchZone{{
			AZID: "use1-az1", RackName: "RACK0",
			Nodes: []model.VectorSearchNode{{ID: 11, InstanceTypeID: 77, Status: "ACTIVE", Up: true}},
		}},
	}, vs)
}
