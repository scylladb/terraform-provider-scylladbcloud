package scylla

import (
	"context"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func (c *Client) GetBackupRetention(ctx context.Context, clusterID int64) (*model.BackupRetention, error) {
	var result model.BackupRetention
	if err := c.get(ctx, c.clusterPath(clusterID, "/backup-retention"), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateBackupRetention changes an existing retention (0-60 days, 0 cancels
// it); the API cannot create one.
func (c *Client) UpdateBackupRetention(ctx context.Context, clusterID, retentionDays int64) (*model.BackupRetention, error) {
	var result model.BackupRetention
	if err := c.patch(ctx, c.clusterPath(clusterID, "/backup-retention"), map[string]int64{"retentionDays": retentionDays}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
