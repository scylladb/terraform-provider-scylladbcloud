package backupretention

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const path = "/account/1/cluster/19/backup-retention"

func retention(days int) map[string]any {
	return map[string]any{"retentionDays": days, "startDate": "2026-10-28T14:43:15Z", "encryptionKeys": []any{"k1"}}
}

func data(t *testing.T, raw map[string]any) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, ResourceBackupRetention().Schema, raw)
}

func TestCreateKeepsMatchingRetention(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, path, retention(1))
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 1})

	require.False(t, resourceBackupRetentionCreate(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "19", d.Id())
	require.Equal(t, "2026-10-28T14:43:15Z", d.Get("start_date"))
	require.Equal(t, []any{"k1"}, d.Get("encryption_keys"))
	require.Empty(t, srv.CallsTo(http.MethodPatch, path))
}

func TestCreateChangesRetention(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, path, retention(1))
	srv.Reply(http.MethodPatch, path, retention(7))
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 7})

	require.False(t, resourceBackupRetentionCreate(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, 7, d.Get("retention_days"))
	calls := srv.CallsTo(http.MethodPatch, path)
	require.Len(t, calls, 1)
	var body map[string]int
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]int{"retentionDays": 7}, body)
}

func TestCreateWithoutRetentionFails(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, path, http.StatusOK, errCodeNotFound)
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 7})

	diags := resourceBackupRetentionCreate(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, "has no backup retention")
	require.Empty(t, d.Id())
}

func TestCreatePatchFailureLeavesNoState(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, path, retention(1))
	srv.Fail(http.MethodPatch, path, http.StatusOK, "045003")
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 7})

	require.True(t, resourceBackupRetentionCreate(context.Background(), d, srv.Client(t)).HasError())
	require.Empty(t, d.Id())
}

func TestReadGoneRemovesState(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, path, http.StatusOK, errCodeNotFound)
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 7})
	d.SetId("19")

	require.False(t, resourceBackupRetentionRead(context.Background(), d, srv.Client(t)).HasError())
	require.Empty(t, d.Id())
}

func TestReadOtherErrorKeepsState(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, path, http.StatusOK, "045001")
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 7})
	d.SetId("19")

	require.True(t, resourceBackupRetentionRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "19", d.Id())
}

func TestUpdateAndDelete(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodPatch, path, retention(30))
	d := data(t, map[string]any{"cluster_id": 19, "retention_days": 30})
	d.SetId("19")
	c := srv.Client(t)

	require.False(t, resourceBackupRetentionUpdate(context.Background(), d, c).HasError())
	require.Len(t, srv.CallsTo(http.MethodPatch, path), 1)

	require.False(t, resourceBackupRetentionDelete(context.Background(), d, c).HasError())
	require.Len(t, srv.Calls(), 1, "destroy must not call the API")
}

func TestImportSetsClusterID(t *testing.T) {
	t.Parallel()
	d := ResourceBackupRetention().TestResourceData()
	d.SetId("19")
	out, err := resourceBackupRetentionImport(context.Background(), d, nil)
	require.NoError(t, err)
	require.Equal(t, 19, out[0].Get("cluster_id"))

	d.SetId("abc")
	_, err = resourceBackupRetentionImport(context.Background(), d, nil)
	require.Error(t, err)
}

func TestRetentionDaysValidation(t *testing.T) {
	t.Parallel()
	s := ResourceBackupRetention().Schema["retention_days"]
	for _, v := range []int{0, 1, 60} {
		require.False(t, s.ValidateDiagFunc(v, nil).HasError(), v)
	}
	for _, v := range []int{-1, 61} {
		require.True(t, s.ValidateDiagFunc(v, nil).HasError(), v)
	}
}
