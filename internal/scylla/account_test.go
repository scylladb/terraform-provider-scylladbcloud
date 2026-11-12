package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestGetDefaultAccount(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/default", map[string]any{"accountId": 1, "userId": 9, "name": "acme", "role": "ADMIN"})

	a, err := srv.Client(t).GetDefaultAccount(context.Background())
	require.NoError(t, err)
	require.Equal(t, &model.Account{AccountID: 1, UserID: 9, Name: "acme", Role: "ADMIN"}, a)
}

func TestGetAccountCertificate(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/certificate", map[string]any{"format": "PEM", "content": "-----BEGIN CERTIFICATE-----"})

	cert, err := srv.Client(t).GetAccountCertificate(context.Background())
	require.NoError(t, err)
	require.Equal(t, &model.ClusterCertificate{Format: "PEM", Content: "-----BEGIN CERTIFICATE-----"}, cert)
}
