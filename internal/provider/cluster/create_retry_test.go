package cluster

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

func TestCreateErrorDiags(t *testing.T) {
	t.Parallel()
	unknown := &scylla.APIError{StatusCode: http.StatusBadGateway, Message: "Bad Gateway"}
	for _, tt := range []struct {
		name       string
		err        error
		clusters   []model.Cluster // nil: no list call expected
		wantDetail string
	}{
		{name: "rejected request", err: &scylla.APIError{StatusCode: http.StatusBadRequest, Message: "bad"}},
		{
			name:       "cluster with the name exists",
			err:        unknown,
			clusters:   []model.Cluster{{ID: 2, ClusterName: "other"}, {ID: 3, ClusterName: "test", Status: "QUEUED"}},
			wantDetail: `terraform import <resource address> 3`,
		},
		{
			name:       "only a deleted cluster has the name",
			err:        unknown,
			clusters:   []model.Cluster{{ID: 3, ClusterName: "test", Status: "DELETED"}},
			wantDetail: `Look for it in ScyllaDB Cloud`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			if tt.clusters != nil {
				srv.Reply(http.MethodGet, "/account/1/clusters", model.Clusters{Clusters: tt.clusters})
			}

			diags := createErrorDiags(context.Background(), srv.Client(t), "test", tt.err)

			require.Len(t, diags, 1)
			require.True(t, diags.HasError())
			if tt.wantDetail == "" {
				require.Empty(t, diags[0].Detail)
			} else {
				require.Contains(t, diags[0].Detail, tt.wantDetail)
			}
		})
	}
}
