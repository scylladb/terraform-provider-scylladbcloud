package allowlistrule

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
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

// TestAllowlistRuleRead reads rule 9 from the clusters of the account. The
// rules of a cluster are read by its ID, also while the cluster has no
// datacenter. The read must not remove the rule from the state while a
// cluster the API could not read may have it.
func TestAllowlistRuleRead(t *testing.T) {
	t.Parallel()

	const (
		// The API omits dcCount for a cluster without a datacenter.
		cluster42 = `{"id":42,"status":"QUEUED"}`
		cluster43 = `{"id":43,"status":"ACTIVE","dcCount":1}`
		// unreadable is what the API sends for a cluster it could not read.
		unreadable = `{"error":{"code":"040023","message":"General Request Error","trace_id":"t1"}}`
	)

	tests := []struct {
		name     string
		clusters []string
		// ruleClusterID is the cluster that has the rule, 0 for none.
		ruleClusterID int
		// wantClusterID is the cluster of the rule read, 0 when the read
		// removes the rule from the state.
		wantClusterID int
		wantErr       string
	}{
		{
			name:          "a cluster has the rule",
			clusters:      []string{cluster43},
			ruleClusterID: 43,
			wantClusterID: 43,
		},
		{
			name:          "the API could not read another cluster",
			clusters:      []string{unreadable, cluster43},
			ruleClusterID: 43,
			wantClusterID: 43,
		},
		{
			// A filter of the clusters without a datacenter would remove
			// this rule from the state.
			name:          "a cluster without a datacenter has the rule",
			clusters:      []string{cluster43, cluster42},
			ruleClusterID: 42,
			wantClusterID: 42,
		},
		{
			name:     "no cluster has the rule",
			clusters: []string{cluster42, cluster43},
		},
		{
			name:     "a cluster the API could not read may have the rule",
			clusters: []string{unreadable, cluster43},
			wantErr: "allowlist rule 9 was not found, but 1 of the clusters of the account could not be read, " +
				"and they may have it: try again later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method+" "+r.URL.Path == "GET /account/7/clusters" {
					_, _ = w.Write([]byte(`{"data":{"clusters":[` + strings.Join(tt.clusters, ",") + `]}}`))
					return
				}

				for _, id := range []int{42, 43} {
					if r.Method+" "+r.URL.Path != fmt.Sprintf("GET /account/7/cluster/%d/network/firewall/allowed", id) {
						continue
					}
					if id != tt.ruleClusterID {
						_, _ = w.Write([]byte(`{"data":[]}`))
						return
					}
					_, _ = fmt.Fprintf(w, `{"data":[{"id":9,"clusterId":%d,"address":"10.0.0.1/32"}]}`, id)
					return
				}

				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}))
			t.Cleanup(srv.Close)

			endpoint, err := url.Parse(srv.URL)
			require.NoError(t, err)

			client := &scylla.Client{
				Endpoint:   endpoint,
				Headers:    make(http.Header),
				HTTPClient: srv.Client(),
				Retry:      retrier.New(nil, nil),
				AccountID:  7,
			}

			d := ResourceAllowlistRule().TestResourceData()
			d.SetId("9")
			require.NoError(t, d.Set("cluster_id", 43))

			diags := resourceAllowlistRuleRead(t.Context(), d, client)

			if tt.wantErr != "" {
				require.True(t, diags.HasError())
				require.Equal(t, tt.wantErr, diags[0].Summary)
				require.Equal(t, "9", d.Id(), "the rule must stay in the state")
				require.Equal(t, 43, d.Get("cluster_id"), "the rule must keep its cluster_id")
				return
			}
			require.False(t, diags.HasError(), "%v", diags)

			if tt.wantClusterID == 0 {
				require.Empty(t, d.Id(), "the rule must be removed from the state")
				return
			}

			require.Equal(t, "9", d.Id())
			require.Equal(t, tt.wantClusterID, d.Get("cluster_id"))
			require.Equal(t, "10.0.0.1/32", d.Get("cidr_block"))
		})
	}
}
