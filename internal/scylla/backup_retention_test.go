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
	require.Equal(t, map[string]any{"retentionDays": float64(14)}, srv.Body(t, http.MethodPatch, path))
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
