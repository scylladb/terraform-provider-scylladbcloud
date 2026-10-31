package scylla

import (
	"context"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// UpdateClusterVPCPeering changes a peering in place. The response is not in
// the documented shape, so the peering is read again.
func (c *Client) UpdateClusterVPCPeering(ctx context.Context, clusterID, peerID int64, req *model.VPCPeeringUpdateRequest) (*model.VPCPeering, error) {
	if err := c.put(ctx, c.clusterPath(clusterID, "/network/vpc/peer/%d", peerID), req, nil); err != nil {
		return nil, err
	}
	return c.GetClusterVPCPeering(ctx, clusterID, peerID)
}
