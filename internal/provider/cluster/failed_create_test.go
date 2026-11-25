package cluster

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const (
	testClusterID = 3
	testRequestID = 9
)

func testCreateRequest(status string) model.ClusterRequest {
	return model.ClusterRequest{
		ID:                  testRequestID,
		ClusterID:           testClusterID,
		RequestType:         "CREATE_CLUSTER",
		Status:              status,
		ProgressPercent:     30,
		ProgressDescription: "Not enough capacity",
	}
}

// testCreateServer answers the calls that Create makes before the wait. The
// first GET of the request returns QUEUED, the next ones return final.
func testCreateServer(t *testing.T, final string) (*scyllatest.Server, *scylla.Client) {
	t.Helper()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/deployment/cloud-provider/1/region/2", model.CloudProviderInstances{
		Instances: []model.CloudProviderInstance{{ID: 4, ExternalID: "i4i.large"}},
	})
	srv.Reply(http.MethodPost, "/account/1/cluster", map[string]any{"requestId": testRequestID})
	srv.Sequence(http.MethodGet, "/account/1/cluster/request/9",
		scyllatest.Response{Data: testCreateRequest("QUEUED")},
		scyllatest.Response{Data: testCreateRequest(final)},
	)

	c := srv.Client(t)
	c.Meta = &scylla.Cloudmeta{
		CloudProviders: []scylla.CloudProvider{{
			CloudProvider: &model.CloudProvider{ID: 1, Name: "AWS"},
			CloudProviderRegions: &model.CloudProviderRegions{
				Regions: []model.CloudProviderRegion{{ID: 2, ExternalID: "us-east-1"}},
			},
		}},
		ScyllaVersions: &model.ScyllaVersions{DefaultScyllaVersionID: 5},
	}
	return srv, c
}

func testClusterData(t *testing.T) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, ResourceCluster().Schema, map[string]any{
		"name":      "test",
		"cloud":     "AWS",
		"region":    "us-east-1",
		"node_type": "i4i.large",
		"min_nodes": 3,
	})
}

// A cluster whose create fails still runs and costs money, so Create must
// keep its ID in state.
// Every failure after the API returned the create request must keep the ID,
// and keep the warnings so far.
func TestCreateKeepsIDWhenALaterStepFails(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		final       string // status of the create request after QUEUED
		setup       func(*scyllatest.Server)
		timeout     time.Duration
		wantErr     string
		wantWarning string
	}{
		{name: "request fails", final: "FAILED", wantErr: "cluster request ID 9 failed (CREATE_CLUSTER at 30%): Not enough capacity"},
		// The deadline can hit in a poll or between polls, so only the prefix is fixed.
		{name: "wait times out", final: "QUEUED", timeout: 50 * time.Millisecond, wantErr: "failed to wait for request 9 creating cluster 3"},
		{
			name:  "cluster read fails",
			final: "COMPLETED",
			setup: func(srv *scyllatest.Server) {
				srv.Fail(http.MethodGet, "/account/1/cluster/3", http.StatusInternalServerError, "Internal Server Error")
			},
			wantErr: "failed to read cluster 3",
		},
		{
			name:  "warning of the encryption fallback is kept",
			final: "FAILED",
			setup: func(srv *scyllatest.Server) {
				srv.Sequence(http.MethodPost, "/account/1/cluster",
					scyllatest.Response{Status: http.StatusBadRequest, Error: "040723"},
					scyllatest.Response{Data: map[string]any{"requestId": testRequestID}},
				)
			},
			wantErr:     "cluster request ID 9 failed",
			wantWarning: "was created without encryption at rest",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv, c := testCreateServer(t, tt.final)
			if tt.setup != nil {
				tt.setup(srv)
			}
			ctx := context.Background()
			if tt.timeout != 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}
			d := testClusterData(t)

			diags := resourceClusterCreate(ctx, d, c)

			require.True(t, diags.HasError())
			require.Contains(t, diagSummaries(diags), tt.wantErr)
			if tt.wantWarning != "" {
				require.Contains(t, diagSummaries(diags), tt.wantWarning)
			}
			require.Equal(t, "3", d.Id())
			require.Equal(t, testRequestID, d.Get("request_id"))
		})
	}
}

func TestReadFailedCreate(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		listed  string // status of the create request in the list
		waitErr bool   // the wait fails with an API error, not with FAILED
	}{
		{name: "failed before read", listed: "FAILED"},
		{name: "fails during the wait", listed: "IN_PROGRESS"},
		{name: "wait fails for another reason", listed: "IN_PROGRESS", waitErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{testCreateRequest(tt.listed)})
			if tt.waitErr {
				srv.Fail(http.MethodGet, "/account/1/cluster/request/9", http.StatusInternalServerError, "000000")
			} else {
				srv.Reply(http.MethodGet, "/account/1/cluster/request/9", testCreateRequest("FAILED"))
			}
			d := testClusterData(t)
			d.SetId("3")

			diags := resourceClusterRead(context.Background(), d, srv.Client(t))

			if tt.waitErr {
				require.True(t, diags.HasError(), "an unclear wait error must stay an error")
				require.NotEqual(t, failedCreateStatus, d.Get("status"))
				return
			}
			require.False(t, diags.HasError(), "%v", diags)
			require.Len(t, diags, 1)
			require.Equal(t, diag.Warning, diags[0].Severity)
			require.Equal(t, "3", d.Id())
			require.Equal(t, failedCreateStatus, d.Get("status"))
			require.Equal(t, testRequestID, d.Get("request_id"))
			require.Empty(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/3"), "Read must not read a cluster whose create failed")
		})
	}
}

// An imported cluster has only its ID in state. Delete needs the name, so
// Read of a failed create takes it from the cluster list.
func TestReadFailedCreateOfImportedCluster(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{testCreateRequest("FAILED")})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/9", testCreateRequest("FAILED"))
	srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: []model.Cluster{
		{ID: 2, ClusterName: "other"},
		{ID: 3, ClusterName: "failed-cluster"},
	}})
	d := ResourceCluster().TestResourceData()
	d.SetId("3")

	diags := resourceClusterRead(context.Background(), d, srv.Client(t))

	require.False(t, diags.HasError(), "%v", diags)
	require.Equal(t, failedCreateStatus, d.Get("status"))
	require.Equal(t, "failed-cluster", d.Get("name"))
}

// Without the name, a later destroy fails before it calls the API. So a
// failed name lookup must fail the refresh, not leave the name empty.
func TestReadFailedCreateOfImportedClusterListError(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, "/account/1/cluster/3/request", []model.ClusterRequest{testCreateRequest("FAILED")})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/9", testCreateRequest("FAILED"))
	srv.Fail(http.MethodGet, "/account/1/clusters", http.StatusInternalServerError, "000000")
	d := ResourceCluster().TestResourceData()
	d.SetId("3")

	diags := resourceClusterRead(context.Background(), d, srv.Client(t))

	require.True(t, diags.HasError())
	require.Contains(t, diagSummaries(diags), "failed to read the name of cluster 3")
	require.Equal(t, "3", d.Id())
}

func diagSummaries(diags diag.Diagnostics) string {
	var s string
	for _, d := range diags {
		s += d.Summary + "\n"
	}
	return s
}
