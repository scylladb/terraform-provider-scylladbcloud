package notificationemails

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const path = "/account/1/notifications/email"

func data(t *testing.T, emails ...any) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, ResourceNotificationEmails().Schema, map[string]any{"emails": emails})
}

func setOf(d *schema.ResourceData) []any {
	return d.Get("emails").(*schema.Set).List()
}

func TestSetPostsSortedEmails(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodPost, path, map[string]any{"email": "", "emails": []any{"a@local.test", "b@local.test"}})
	d := data(t, "b@local.test", "a@local.test")

	require.False(t, resourceNotificationEmailsSet(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "1", d.Id())
	require.ElementsMatch(t, []any{"a@local.test", "b@local.test"}, setOf(d))

	calls := srv.CallsTo(http.MethodPost, path)
	require.Len(t, calls, 1)
	var body map[string][]string
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, []string{"a@local.test", "b@local.test"}, body["emails"])
}

func TestSetRejected(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodPost, path, http.StatusOK, "010806")
	d := data(t, "not-an-email")

	require.True(t, resourceNotificationEmailsSet(context.Background(), d, srv.Client(t)).HasError())
	require.Empty(t, d.Id())
}

func TestRead(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, path, map[string]any{"email": "a@local.test", "emails": []any{"a@local.test"}, "clusterNotifications": []any{}})
	d := data(t, "x@local.test")
	d.SetId("1")

	require.False(t, resourceNotificationEmailsRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, []any{"a@local.test"}, setOf(d))
}

func TestReadNoEmailsRemovesState(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]func(*scyllatest.Server){
		"error 010808": func(s *scyllatest.Server) { s.Fail(http.MethodGet, path, http.StatusOK, errCodeNoEmails) },
		"empty list":   func(s *scyllatest.Server) { s.Reply(http.MethodGet, path, map[string]any{"emails": []any{}}) },
	} {
		srv := scyllatest.New(t)
		reply(srv)
		d := data(t, "a@local.test")
		d.SetId("1")
		require.False(t, resourceNotificationEmailsRead(context.Background(), d, srv.Client(t)).HasError(), name)
		require.Empty(t, d.Id(), name)
	}
}

func TestReadOtherErrorFails(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Fail(http.MethodGet, path, http.StatusOK, "010001")
	d := data(t, "a@local.test")
	d.SetId("1")
	require.True(t, resourceNotificationEmailsRead(context.Background(), d, srv.Client(t)).HasError())
}

func TestDeleteWarnsAndCallsNothing(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	diags := resourceNotificationEmailsDelete(context.Background(), data(t, "a@local.test"), srv.Client(t))
	require.Len(t, diags, 1)
	require.Equal(t, diag.Warning, diags[0].Severity)
	require.Empty(t, srv.Calls())
}

func TestEmailsNeedAtLeastOne(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, ResourceNotificationEmails().Schema["emails"].MinItems)
}
