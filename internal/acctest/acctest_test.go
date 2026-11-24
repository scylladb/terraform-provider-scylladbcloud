package acctest

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const (
	testRequestsPath = "/account/1/cluster/3/request"
	testRequestPath  = "/account/1/cluster/request/9"
)

func testDeleteRequest(status string) model.ClusterRequest {
	return model.ClusterRequest{ID: 9, ClusterID: 3, RequestType: "DELETE_CLUSTER", Status: status}
}

// The destroy check is the only guard against clusters that a test leaves
// running. It must pass only when the API confirms that a cluster is deleted.
func TestCheckClusterDestroyed(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		requests scyllatest.Response   // the list of delete requests
		request  []scyllatest.Response // the polls of the newest one
		wantErr  string                // "" when the check must pass
	}{
		{name: "not found", requests: scyllatest.Response{Status: http.StatusNotFound, Error: "Not Found"}},
		{name: "cluster deleted", requests: scyllatest.Response{Status: http.StatusBadRequest, Error: "CLUSTER_DELETED"}},
		{name: "deleted code", requests: scyllatest.Response{Status: http.StatusBadRequest, Error: "040001"}},
		{
			name:     "delete completed",
			requests: scyllatest.Response{Data: []model.ClusterRequest{testDeleteRequest("COMPLETED")}},
			request:  []scyllatest.Response{{Data: testDeleteRequest("COMPLETED")}},
		},
		{
			name:     "delete still in progress",
			requests: scyllatest.Response{Data: []model.ClusterRequest{testDeleteRequest("IN_PROGRESS")}},
			request: []scyllatest.Response{
				{Data: testDeleteRequest("IN_PROGRESS")},
				{Data: testDeleteRequest("COMPLETED")},
			},
		},
		{
			name: "newest delete request counts",
			requests: scyllatest.Response{Data: []model.ClusterRequest{
				{ID: 9, ClusterID: 3, RequestType: "DELETE_CLUSTER", Status: "IN_PROGRESS"},
				{ID: 4, ClusterID: 3, RequestType: "DELETE_CLUSTER", Status: "FAILED"},
			}},
			request: []scyllatest.Response{{Data: testDeleteRequest("COMPLETED")}},
		},
		{
			name:     "delete failed",
			requests: scyllatest.Response{Data: []model.ClusterRequest{testDeleteRequest("IN_PROGRESS")}},
			request:  []scyllatest.Response{{Data: testDeleteRequest("FAILED")}},
			wantErr:  "cannot confirm that cluster 3 is destroyed: cluster request ID 9 failed",
		},
		{
			name:     "no delete request",
			requests: scyllatest.Response{Data: []model.ClusterRequest{}},
			wantErr:  "cluster 3 still exists, no delete request was sent",
		},
		{name: "server error", requests: scyllatest.Response{Status: http.StatusInternalServerError, Error: "Internal Server Error"}, wantErr: "cannot confirm that cluster 3 is destroyed"},
		{name: "expired token", requests: scyllatest.Response{Status: http.StatusUnauthorized, Error: "Unauthorized"}, wantErr: "cannot confirm that cluster 3 is destroyed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Sequence(http.MethodGet, testRequestsPath, tt.requests)
			if tt.request != nil {
				srv.Sequence(http.MethodGet, testRequestPath, tt.request...)
			}

			s := &terraform.State{Modules: []*terraform.ModuleState{{
				Path: []string{"root"},
				Resources: map[string]*terraform.ResourceState{
					"scylladbcloud_cluster.test":        {Type: "scylladbcloud_cluster", Primary: &terraform.InstanceState{ID: "3"}},
					"scylladbcloud_allowlist_rule.test": {Type: "scylladbcloud_allowlist_rule", Primary: &terraform.InstanceState{ID: "57"}},
				},
			}}}

			err := checkClusterDestroyed(context.Background(), srv.Client(t), s)

			// A cluster that is being deleted can have no datacenter, and
			// then GetCluster fails without an API error.
			require.Empty(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/3"), "the check must not read the cluster")
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
