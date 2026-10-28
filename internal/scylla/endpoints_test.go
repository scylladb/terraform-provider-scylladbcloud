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

// clusterRequestJSON is a cluster request as the public API returns it.
var clusterRequestJSON = map[string]any{"id": 42, "requestType": "X", "accountId": 1, "clusterId": 3, "status": "QUEUED"}

func body(t *testing.T, srv *scyllatest.Server, method, path string) map[string]any {
	t.Helper()
	calls := srv.CallsTo(method, path)
	require.Len(t, calls, 1, "%s %s", method, path)
	var got map[string]any
	scyllatest.JSON(t, calls[0].Body, &got)
	return got
}

func TestClusterRequestEndpoints(t *testing.T) {
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
		{
			name:   "create vector search",
			method: http.MethodPost,
			path:   "/account/1/cluster/3/dc/5/vector-search",
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.CreateVectorSearch(context.Background(), 3, 5, &model.VectorSearchRequest{NodeCount: 2, DefaultInstanceTypeID: 77})
			},
			wantBody: map[string]any{"nodeCount": float64(2), "defaultInstanceTypeId": float64(77)},
		},
		{
			name:   "update vector search",
			method: http.MethodPatch,
			path:   "/account/1/cluster/3/dc/5/vector-search",
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.UpdateVectorSearch(context.Background(), 3, 5, &model.VectorSearchRequest{NodeCount: 4, DefaultInstanceTypeID: 77})
			},
			wantBody: map[string]any{"nodeCount": float64(4), "defaultInstanceTypeId": float64(77)},
		},
		{
			name:   "recreate connection",
			method: http.MethodPut,
			path:   "/account/1/cluster/3/network/vpc/connection/9",
			call: func(c *scylla.Client) (*model.ClusterRequest, error) {
				return c.RecreateClusterConnection(context.Background(), 3, 9, &model.ClusterConnectionRecreateRequest{
					Name: "tgw", CIDRList: []string{"10.1.0.0/16"}, Data: map[string]string{"tgwid": "tgw-1"}, Status: "ACTIVE",
				})
			},
			wantBody: map[string]any{"name": "tgw", "cidrList": []any{"10.1.0.0/16"}, "data": map[string]any{"tgwid": "tgw-1"}, "status": "ACTIVE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Reply(tt.method, tt.path, clusterRequestJSON)

			r, err := tt.call(srv.Client(t))
			require.NoError(t, err)
			require.Equal(t, int64(42), r.ID)
			require.Equal(t, int64(3), r.ClusterID)
			require.Equal(t, "QUEUED", r.Status)
			require.Equal(t, tt.wantBody, body(t, srv, tt.method, tt.path))
		})
	}
}

func TestDeleteVectorSearch(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/cluster/3/dc/5/vector-search"
	srv.Reply(http.MethodDelete, path, clusterRequestJSON)

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

func TestBackupRetention(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/cluster/3/backup-retention"
	retention := map[string]any{"retentionDays": 14, "startDate": "2026-10-28T00:00:00Z", "encryptionKeys": []any{"k1"}}
	srv.Reply(http.MethodGet, path, retention)
	srv.Reply(http.MethodPatch, path, retention)
	c := srv.Client(t)

	want := &model.BackupRetention{RetentionDays: 14, StartDate: "2026-10-28T00:00:00Z", EncryptionKeys: []string{"k1"}}
	got, err := c.GetBackupRetention(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, want, got)

	got, err = c.UpdateBackupRetention(context.Background(), 3, 14)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, map[string]any{"retentionDays": float64(14)}, body(t, srv, http.MethodPatch, path))
}

func TestBackupRetentionNotFound(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, "/account/1/cluster/3/backup-retention", http.StatusOK, "040002")

	_, err := srv.Client(t).GetBackupRetention(context.Background(), 3)
	require.Error(t, err)
	var apiErr *scylla.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "040002", apiErr.Code)
}

func TestUpdateClusterVPCPeeringReadsBack(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/cluster/3/network/vpc/peer/8"
	// The PUT answers with Siren's PascalCase struct, which is ignored.
	srv.Reply(http.MethodPut, path, map[string]any{"ID": 8, "PeerVPCIPv4CIDRList": []any{"10.2.0.0/16"}})
	srv.Reply(http.MethodGet, path, map[string]any{"id": 8, "status": "ACTIVE", "cidrList": []any{"10.2.0.0/16", "10.3.0.0/16"}})

	p, err := srv.Client(t).UpdateClusterVPCPeering(context.Background(), 3, 8, &model.VPCPeeringUpdateRequest{
		Status: "ACTIVE", CIDRBlock: "10.2.0.0/16,10.3.0.0/16",
	})
	require.NoError(t, err)
	require.Equal(t, int64(8), p.ID)
	require.Equal(t, []string{"10.2.0.0/16", "10.3.0.0/16"}, p.CIDRList)
	require.Equal(t, map[string]any{"status": "ACTIVE", "cidrBlock": "10.2.0.0/16,10.3.0.0/16"}, body(t, srv, http.MethodPut, path))
}

func TestNotificationEmails(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	const path = "/account/1/notifications/email"
	srv.Reply(http.MethodGet, path, map[string]any{"email": "a@local.test", "emails": []any{"a@local.test", "b@local.test"}})
	srv.Reply(http.MethodPost, path, map[string]any{"email": "", "emails": []any{"c@local.test"}})
	c := srv.Client(t)

	got, err := c.GetAccountNotificationEmails(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"a@local.test", "b@local.test"}, got)

	got, err = c.SetAccountNotificationEmails(context.Background(), []string{"c@local.test"})
	require.NoError(t, err)
	require.Equal(t, []string{"c@local.test"}, got)
	require.Equal(t, map[string]any{"emails": []any{"c@local.test"}}, body(t, srv, http.MethodPost, path))
}

func TestGetAccountCertificate(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/certificate", map[string]any{"format": "PEM", "content": "-----BEGIN CERTIFICATE-----"})

	cert, err := srv.Client(t).GetAccountCertificate(context.Background())
	require.NoError(t, err)
	require.Equal(t, &model.ClusterCertificate{Format: "PEM", Content: "-----BEGIN CERTIFICATE-----"}, cert)
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
