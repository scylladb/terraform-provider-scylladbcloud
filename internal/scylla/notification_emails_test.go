package scylla_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

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
	require.Equal(t, map[string]any{"emails": []any{"c@local.test"}}, srv.Body(t, http.MethodPost, path))
}
