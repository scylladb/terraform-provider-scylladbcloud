package scylla

import (
	"context"
	"net/http"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func (c *Client) GetVectorSearch(ctx context.Context, clusterID, dcID int64) (*model.VectorSearch, error) {
	var result model.VectorSearch
	if err := c.get(ctx, c.clusterPath(clusterID, "/dc/%d/vector-search", dcID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateVectorSearch(ctx context.Context, clusterID, dcID int64, req *model.VectorSearchRequest) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	if err := c.post(ctx, c.clusterPath(clusterID, "/dc/%d/vector-search", dcID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateVectorSearch(ctx context.Context, clusterID, dcID int64, req *model.VectorSearchRequest) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	if err := c.patch(ctx, c.clusterPath(clusterID, "/dc/%d/vector-search", dcID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteVectorSearch(ctx context.Context, clusterID, dcID int64) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	if err := c.retryCall(ctx, http.MethodDelete, c.clusterPath(clusterID, "/dc/%d/vector-search", dcID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
