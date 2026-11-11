package scylla

import (
	"context"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// RecreateClusterConnection replaces a connection's revision (not supported
// for AWS PrivateLink or GCP PSC); wait for the returned request.
func (c *Client) RecreateClusterConnection(ctx context.Context, clusterID, connectionID int64, req *model.ClusterConnectionRecreateRequest) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	if err := c.put(ctx, c.clusterPath(clusterID, "/network/vpc/connection/%d", connectionID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
