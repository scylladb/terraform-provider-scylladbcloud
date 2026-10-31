package datasource

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestDataSourceAccount(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/default", map[string]any{
		"accountId": 2, "userId": 7, "name": "TF Dev", "userAccountStatus": "ACTIVE", "role": "ADMIN", "accountStatus": "ACTIVE",
	})
	d := DataSourceAccount().TestResourceData()

	require.False(t, dataSourceAccountRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "2", d.Id())
	require.Equal(t, 2, d.Get("account_id"))
	require.Equal(t, "TF Dev", d.Get("name"))
	require.Equal(t, "ACTIVE", d.Get("status"))
	require.Equal(t, "ADMIN", d.Get("role"))
	require.Equal(t, 7, d.Get("user_id"))
}

func TestDataSourceAccountCertificate(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/certificate", map[string]any{"format": "PEM", "content": "-----BEGIN CERTIFICATE-----"})
	d := DataSourceAccountCertificate().TestResourceData()

	require.False(t, dataSourceAccountCertificateRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "1", d.Id())
	require.Equal(t, "PEM", d.Get("format"))
	require.Equal(t, "-----BEGIN CERTIFICATE-----", d.Get("content"))
}

func TestDataSourceAccountCertificateError(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, "/account/1/certificate", http.StatusOK, "UNKNOWN_ERROR")
	require.True(t, dataSourceAccountCertificateRead(context.Background(), DataSourceAccountCertificate().TestResourceData(), srv.Client(t)).HasError())
}
