package connection

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const connPath = "/account/1/cluster/3/network/vpc/connection/9"

func connData(t *testing.T, typ string) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, ResourceClusterConnection().Schema, map[string]any{
		"name": "tgw", "cluster_id": 3, "datacenter": "AWS_US_EAST_1", "type": typ,
		"cidrlist": []any{"10.1.0.0/16"}, "status": "ACTIVE",
		"data": map[string]any{"tgwid": "tgw-2", "ramarn": "arn:aws:ram::1:resource-share/x"},
	})
}

func TestRecreateConnectionSendsWholeRevision(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodPut, connPath, model.ClusterRequest{ID: 80, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/80", model.ClusterRequest{ID: 80, Status: "COMPLETED"})

	diags := recreateClusterConnection(context.Background(), srv.Client(t), connData(t, "AWS_TGW_ATTACHMENT"), 3, 9, []string{"10.1.0.0/16"})
	require.False(t, diags.HasError(), "%v", diags)

	calls := srv.CallsTo(http.MethodPut, connPath)
	require.Len(t, calls, 1)
	var body map[string]any
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]any{
		"name": "tgw", "cidrList": []any{"10.1.0.0/16"}, "status": "ACTIVE",
		"data": map[string]any{"tgwid": "tgw-2", "ramarn": "arn:aws:ram::1:resource-share/x"},
	}, body)
	require.Len(t, srv.CallsTo(http.MethodGet, "/account/1/cluster/request/80"), 1)
}

func TestRecreateConnectionFailedRequest(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodPut, connPath, model.ClusterRequest{ID: 80, Status: "QUEUED"})
	srv.Reply(http.MethodGet, "/account/1/cluster/request/80", model.ClusterRequest{ID: 80, Status: "FAILED"})

	diags := recreateClusterConnection(context.Background(), srv.Client(t), connData(t, "AWS_VPC_PEERING"), 3, 9, []string{"10.1.0.0/16"})
	require.True(t, diags.HasError())
}

func TestRecreateConnectionRejectsPrivateLinkTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"AWS_PRIVATE_LINK", "GCP_PRIVATE_SERVICE_CONNECT"} {
		srv := scyllatest.New(t)
		diags := recreateClusterConnection(context.Background(), srv.Client(t), connData(t, typ), 3, 9, nil)
		require.True(t, diags.HasError(), typ)
		require.Contains(t, diags[0].Summary, typ)
		require.Empty(t, srv.Calls(), typ)
	}
}

func TestConnectionStatusValidation(t *testing.T) {
	t.Parallel()
	validate := ResourceClusterConnection().Schema["status"].ValidateDiagFunc
	for _, v := range []string{"ACTIVE", "INACTIVE"} {
		require.False(t, validate(v, cty.Path{}).HasError(), v)
	}
	for _, v := range []string{"INACTIVE ", "active", ""} {
		require.True(t, validate(v, cty.Path{}).HasError(), v)
	}
}
