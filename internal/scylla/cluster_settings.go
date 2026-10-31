package scylla

import (
	"context"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// RenameCluster starts renaming a cluster; wait for the returned request.
func (c *Client) RenameCluster(ctx context.Context, clusterID int64, name string) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	if err := c.patch(ctx, c.clusterPath(clusterID, "/name"), map[string]string{"name": name}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetClientEncryptionMode starts switching client-to-node encryption to mode
// (model.ClientEncryptionModeEncrypted or Both). The API refuses the mode the
// cluster already has.
func (c *Client) SetClientEncryptionMode(ctx context.Context, clusterID int64, mode string) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	if err := c.put(ctx, c.clusterPath(clusterID, "/encryption/mode"), map[string]string{"mode": mode}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
