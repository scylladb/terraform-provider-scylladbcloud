package cluster

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// TestResourceClusterUpdateMinNodesInstance changes min_nodes of a Standard
// cluster to 6. While the datacenter has no ACTIVE node, the API reports
// instance ID 0 and the read keeps min_nodes from the state, so a plan can
// still change it. The resize needs the instance, so the update must not
// send one with instance type 0.
func TestResourceClusterUpdateMinNodesInstance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		instanceID int
		nodes      string
		// wantResize is the resize request, "" for none.
		wantResize string
		wantErr    string
	}{
		{
			name:       "no node is ACTIVE",
			instanceID: 0,
			nodes:      `[]`,
			wantErr:    "cluster 42 has no ACTIVE node, so its node_type is unknown: try again when it has ACTIVE nodes",
		},
		{
			// The fake API fails the resize, so the update does not wait
			// for it.
			name:       "three nodes are ACTIVE",
			instanceID: 2,
			nodes:      `[{"id":1,"status":"ACTIVE"},{"id":2,"status":"ACTIVE"},{"id":3,"status":"ACTIVE"}]`,
			wantResize: `{"dcNodes":[{"dcId":5,"instanceTypeId":2,"wantedSize":6}]}`,
			wantErr:    "error resizing cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dc := fmt.Sprintf(`{"id":5,"name":"AWS_US_EAST_1","instanceId":%d}`, tt.instanceID)

			var (
				mu      sync.Mutex
				resizes []string
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /account/7/cluster/42":
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":42,"clusterName":"cluster","status":"ACTIVE",` +
						`"cloudProviderId":1,"region":{"id":1,"externalId":"us-east-1"},` +
						`"instance":` + placeholderInstance + `,"dc":` + dc + `,"dataCenters":[` + dc + `],` +
						`"nodes":` + tt.nodes + `}}}`))
				case "GET /account/7/cluster/42/dc/5":
					_, _ = w.Write([]byte(`{"data":` + dc + `}`))
				case "GET /account/7/cluster/42/request":
					// No request is in progress.
					_, _ = w.Write([]byte(`{"data":[]}`))
				case "POST /account/7/cluster/42/resize":
					body, _ := io.ReadAll(r.Body)
					mu.Lock()
					resizes = append(resizes, string(body))
					mu.Unlock()

					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":"041002"}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			t.Cleanup(srv.Close)

			client := newTestClient(t, srv)

			// A diff from no state, so the new value of min_nodes is 6.
			d := schema.TestResourceDataRaw(t, ResourceCluster().Schema, map[string]any{"min_nodes": 6})
			d.SetId("42")

			diags := resourceClusterUpdateMinNodes(t.Context(), d, client, client)

			require.True(t, diags.HasError())
			require.Contains(t, diags[0].Summary, tt.wantErr)

			mu.Lock()
			defer mu.Unlock()
			if tt.wantResize == "" {
				require.Empty(t, resizes, "the update must not resize the cluster")
				return
			}
			require.Len(t, resizes, 1)
			require.JSONEq(t, tt.wantResize, resizes[0])
		})
	}
}

// TestResourceClusterUpdateMinNodesState scales a six-node Standard cluster
// in to 3 through the SDK, and fails the update. The SDK stores the planned
// values of a failed update unless the update says otherwise, and the read
// keeps a min_nodes below the node count, so a stored 3 would plan no change
// once six nodes are ACTIVE. A failed check that decides whether to resize
// keeps 6. An error of the resize call stores 3: the API can save the request
// and still answer with an error, and planning the resize again could send it
// twice. The same apply raises backup_retention_days from 0, which the update
// stores without an API call and the delete reads from the state. Every failed
// update must store the new value: with 0 in the state, a destroy after it
// would delete the backups at once.
func TestResourceClusterUpdateMinNodesState(t *testing.T) {
	t.Parallel()

	const (
		retentionDays = 14
		realInstance  = `{"id":2,"externalId":"i3.large","totalStorage":475}`
		sixNodes      = `[{"id":1,"status":"ACTIVE"},{"id":2,"status":"ACTIVE"},{"id":3,"status":"ACTIVE"},` +
			`{"id":4,"status":"ACTIVE"},{"id":5,"status":"ACTIVE"},{"id":6,"status":"ACTIVE"}]`
	)

	tests := []struct {
		name string
		// active is whether the nodes are ACTIVE during the plan and the
		// update. While none is, the API reports instance ID 0.
		active bool
		// failList fails the list of IN_PROGRESS requests that the update
		// waits on.
		failList bool
		wantErr  string
		// wantMinNodes is min_nodes in the state after the failed update.
		wantMinNodes string
		wantResizes  int32
	}{
		{
			name:         "no node is ACTIVE",
			wantErr:      "cluster 42 has no ACTIVE node, so its node_type is unknown: try again when it has ACTIVE nodes",
			wantMinNodes: "6",
		},
		{
			name:         "the request list fails",
			failList:     true,
			wantErr:      "failed waiting for no in-progress cluster requests for cluster 42",
			wantMinNodes: "6",
		},
		{
			name:         "the resize call fails",
			active:       true,
			wantErr:      "error resizing cluster",
			wantMinNodes: "3",
			wantResizes:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				active  atomic.Bool
				resizes atomic.Int32
			)
			active.Store(tt.active)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				instanceID, instance, nodes := 0, placeholderInstance, `[]`
				if active.Load() {
					instanceID, instance, nodes = 2, realInstance, sixNodes
				}
				dc := fmt.Sprintf(`{"id":5,"name":"AWS_US_EAST_1","instanceId":%d}`, instanceID)

				switch r.Method + " " + r.URL.Path {
				case "GET /account/7/cluster/42/request":
					// The read reads the create request, and the update waits
					// until no request is IN_PROGRESS.
					switch {
					case r.URL.Query().Get("status") != "IN_PROGRESS":
						_, _ = w.Write([]byte(`{"data":[{"id":11,"status":"COMPLETED"}]}`))
					case tt.failList:
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"error":"040023"}`))
					default:
						_, _ = w.Write([]byte(`{"data":[]}`))
					}
				case "GET /account/7/cluster/42":
					_, _ = w.Write([]byte(`{"data":{"cluster":{"id":42,"clusterName":"cluster","status":"ACTIVE",` +
						`"cloudProviderId":1,"userApiInterface":"CQL","broadcastType":"PRIVATE","dns":true,` +
						`"region":{"id":1,"externalId":"us-east-1"},"scyllaVersion":{"version":"2025.1.0"},` +
						`"instance":` + instance + `,"dc":` + dc + `,"dataCenters":[` + dc + `],` +
						`"nodes":` + nodes + `}}}`))
				case "GET /account/7/cluster/42/dc/5":
					_, _ = w.Write([]byte(`{"data":` + dc + `}`))
				case "GET /deployment/cloud-provider/1/region/1":
					_, _ = w.Write([]byte(`{"data":{"instances":[` + realInstance + `]}}`))
				case "GET /account/7/cluster/42/certificate":
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":"041201"}`))
				case "POST /account/7/cluster/42/resize":
					resizes.Add(1)
					// The general resize error, which the API also sends after
					// it saved the request. It answers with status 200.
					_, _ = w.Write([]byte(`{"error":"041002"}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			t.Cleanup(srv.Close)

			client := newTestClient(t, srv)
			client.Meta = &scylla.Cloudmeta{CloudProviders: []scylla.CloudProvider{{
				CloudProvider:        &model.CloudProvider{ID: 1, Name: "AWS"},
				CloudProviderRegions: &model.CloudProviderRegions{},
			}}}

			// The refresh of the plan keeps min_nodes = 6.
			d := ResourceCluster().TestResourceData()
			d.SetId("42")
			for k, v := range map[string]any{
				"name":           "cluster",
				"cloud":          "AWS",
				"region":         "us-east-1",
				"node_type":      "i3.large",
				"node_disk_size": 475,
				"min_nodes":      6,
				// Backups are deleted at once with the cluster.
				"backup_retention_days": 0,
				// The create stores the default, and the read of a CQL
				// cluster keeps it.
				"alternator_write_isolation": "only_rmw_uses_lwt",
			} {
				require.NoError(t, d.Set(k, v))
			}

			state, diags := ResourceCluster().RefreshWithoutUpgrade(t.Context(), d.State(), client)
			require.False(t, diags.HasError(), "%v", diags)

			config := map[string]cty.Value{
				"name":           cty.StringVal("cluster"),
				"cloud":          cty.StringVal("AWS"),
				"region":         cty.StringVal("us-east-1"),
				"node_type":      cty.StringVal("i3.large"),
				"node_disk_size": cty.NumberIntVal(475),
				"min_nodes":      cty.NumberIntVal(3),
				// The same apply keeps the backups for some days.
				"backup_retention_days": cty.NumberIntVal(retentionDays),
			}

			// requireScaleIn checks that the plan of config scales the
			// cluster in.
			requireScaleIn := func(state *terraform.InstanceState) *terraform.InstanceDiff {
				t.Helper()

				diff, err := clusterDiff(t, state, config)
				require.NoError(t, err)
				require.NotNil(t, diff)
				require.False(t, diff.RequiresNew())

				a := diff.Attributes["min_nodes"]
				require.NotNil(t, a)
				require.Equal(t, []string{"6", "3"}, []string{a.Old, a.New})
				return diff
			}

			diff := requireScaleIn(state)

			state, diags = ResourceCluster().Apply(t.Context(), state, diff, client)
			require.True(t, diags.HasError())
			require.Contains(t, diags[0].Summary, tt.wantErr)
			require.Equal(t, tt.wantMinNodes, state.Attributes["min_nodes"])
			require.Equal(t, strconv.Itoa(retentionDays), state.Attributes["backup_retention_days"])
			require.Equal(t, tt.wantResizes, resizes.Load())

			// A sent resize is the API's to carry out.
			if tt.wantResizes > 0 {
				return
			}

			active.Store(true)

			state, diags = ResourceCluster().RefreshWithoutUpgrade(t.Context(), state, client)
			require.False(t, diags.HasError(), "%v", diags)

			// The plan still scales the cluster in once its nodes are ACTIVE.
			requireScaleIn(state)
		})
	}
}

// newTestClient returns a client of the fake API srv for account 7. It is
// made by NewClient, as the provider's client is, but polls every 5 ms and
// does not retry, so srv sees each request once.
func newTestClient(t *testing.T, srv *httptest.Server) *scylla.Client {
	t.Helper()

	client, err := scylla.NewClient(srv.URL, "token", "test", "", false)
	require.NoError(t, err)

	client.HTTPClient = srv.Client()
	client.Retry = retrier.New(nil, nil)
	client.PollInterval = 5 * time.Millisecond
	client.AccountID = 7

	return client
}
