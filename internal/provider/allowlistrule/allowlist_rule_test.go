package allowlistrule

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

// An imported rule starts from its ID alone, so Read must set every attribute.
func TestReadSetsEveryAttribute(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/clusters", map[string]any{"clusters": []any{map[string]any{"id": 3}}})
	srv.Reply(http.MethodGet, "/account/1/cluster/3/network/firewall/allowed", []any{
		map[string]any{"id": 57, "clusterId": 3, "address": "192.0.2.20/32"},
	})
	d := ResourceAllowlistRule().TestResourceData()
	d.SetId("57")

	require.False(t, resourceAllowlistRuleRead(context.Background(), d, srv.Client(t)).HasError())
	require.Equal(t, "57", d.Id())
	require.Equal(t, 57, d.Get("rule_id"))
	require.Equal(t, 3, d.Get("cluster_id"))
	require.Equal(t, "192.0.2.20/32", d.Get("cidr_block"))
}
