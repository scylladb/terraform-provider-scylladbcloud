package vpcpeering

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const peerPath = "/account/1/cluster/3/network/vpc/peer/8"

// peeringData is peering 8 of cluster 3 in state with one CIDR, handed to
// Update with cfg as the configuration.
func peeringData(t *testing.T, cfg map[string]interface{}) (*schema.ResourceData, *terraform.InstanceDiff) {
	t.Helper()
	r := ResourceVPCPeering()
	state := &terraform.InstanceState{ID: "pcx-1", Meta: map[string]interface{}{"schema_version": "1"}, Attributes: map[string]string{
		"id": "pcx-1", "cluster_id": "3", "vpc_peering_id": "8", "datacenter": "AWS_US_EAST_1",
		"peer_vpc_id": "vpc-1", "peer_region": "us-east-1", "peer_account_id": "123", "allow_cql": "true",
		"peer_cidr_blocks.#": "1", "peer_cidr_blocks.0": "10.2.0.0/16",
	}}
	base := map[string]interface{}{
		"cluster_id": 3, "datacenter": "AWS_US_EAST_1", "peer_vpc_id": "vpc-1", "peer_region": "us-east-1",
		"peer_account_id": "123", "allow_cql": true, "peer_cidr_blocks": []interface{}{"10.2.0.0/16"},
	}
	for k, v := range cfg {
		base[k] = v
	}
	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(base), nil)
	require.NoError(t, err)
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	require.NoError(t, err)
	return d, diff
}

func TestPeerCIDRChangeIsInPlace(t *testing.T) {
	t.Parallel()
	require.False(t, ResourceVPCPeering().Schema["peer_cidr_blocks"].ForceNew)
	_, diff := peeringData(t, map[string]interface{}{"peer_cidr_blocks": []interface{}{"10.2.0.0/16", "10.3.0.0/16"}})
	require.NotNil(t, diff)
	require.False(t, diff.RequiresNew())
}

func TestUpdatePeerCIDRs(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Sequence(http.MethodGet, peerPath,
		scyllatest.Response{Data: map[string]any{"id": 8, "status": "ACTIVE", "cidrList": []any{"10.2.0.0/16"}}},
		scyllatest.Response{Data: map[string]any{"id": 8, "status": "ACTIVE", "cidrList": []any{"10.2.0.0/16", "10.3.0.0/16"}}},
	)
	srv.Reply(http.MethodPut, peerPath, map[string]any{"ID": 8})
	d, _ := peeringData(t, map[string]interface{}{"peer_cidr_blocks": []interface{}{"10.2.0.0/16", "10.3.0.0/16"}})

	diags := resourceVPCPeeringUpdate(context.Background(), d, srv.Client(t))
	require.False(t, diags.HasError(), "%v", diags)
	calls := srv.CallsTo(http.MethodPut, peerPath)
	require.Len(t, calls, 1)
	var body map[string]string
	scyllatest.JSON(t, calls[0].Body, &body)
	require.Equal(t, map[string]string{"status": "ACTIVE", "cidrBlock": "10.2.0.0/16,10.3.0.0/16"}, body)
	require.Equal(t, []any{"10.2.0.0/16", "10.3.0.0/16"}, d.Get("peer_cidr_blocks"))
}

func TestUpdatePeerCIDRsNeedsSettledPeering(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Reply(http.MethodGet, peerPath, map[string]any{"id": 8, "status": "PENDING_ACCEPTANCE"})
	d, _ := peeringData(t, map[string]interface{}{"peer_cidr_blocks": []interface{}{"10.3.0.0/16"}})

	diags := resourceVPCPeeringUpdate(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, "PENDING_ACCEPTANCE")
	require.Empty(t, srv.CallsTo(http.MethodPut, peerPath))
}

func TestUpdateAllowCQLStillUnsupported(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	d, _ := peeringData(t, map[string]interface{}{"allow_cql": false})

	diags := resourceVPCPeeringUpdate(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, "allow_cql")
	require.Empty(t, srv.Calls())
}

func TestUpdateEmptyCIDRsWithoutGCPDefault(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	d, _ := peeringData(t, map[string]interface{}{"peer_cidr_blocks": []interface{}{}})

	diags := resourceVPCPeeringUpdate(context.Background(), d, srv.Client(t))
	require.True(t, diags.HasError())
	require.Contains(t, diags[0].Summary, "cannot be empty")
	require.Empty(t, srv.Calls())
}
