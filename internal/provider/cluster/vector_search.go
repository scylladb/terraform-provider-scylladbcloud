package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

const installVectorSearchRequestType = "INSTALL_VECTOR_SEARCH"

var errVectorSearchChanged = errors.New("vector_search is applied only when the cluster is created; " +
	"use the scylladbcloud_vector_search resource to add or resize vector search on an existing cluster, " +
	"or remove the vector_search block")

// vectorSearchBootstrap resolves the block's node_type to its instance type ID
// in the cluster's region.
func vectorSearchBootstrap(ctx context.Context, c *scylla.Client, providerID, regionID int64, block map[string]interface{}) (*model.VectorSearchBootstrap, error) {
	nodeType, _ := block["node_type"].(string)
	nodeCount, _ := block["node_count"].(int)

	instances, err := c.ListVectorSearchInstances(ctx, providerID, regionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vector search instance types: %w", err)
	}
	names := make([]string, 0, len(instances))
	for _, i := range instances {
		if strings.EqualFold(i.ExternalID, nodeType) {
			return &model.VectorSearchBootstrap{NodeCount: int64(nodeCount), DefaultInstanceTypeID: i.ID}, nil
		}
		names = append(names, i.ExternalID)
	}
	return nil, fmt.Errorf("vector_search node_type %q is not available in this region; available: %s", nodeType, strings.Join(names, ", "))
}

// awaitVectorSearchInstall waits for the install request the API queues once
// a cluster created with vector search has bootstrapped.
func awaitVectorSearchInstall(ctx context.Context, c *scylla.Client, clusterID int64) error {
	reqs, err := c.ListClusterRequest(ctx, clusterID, scylla.ListClusterRequestParams{Type: installVectorSearchRequestType})
	if err != nil {
		return fmt.Errorf("failed to list cluster requests: %w", err)
	}
	if len(reqs) == 0 {
		return fmt.Errorf("no %s request was found", installVectorSearchRequestType)
	}
	for _, r := range reqs {
		if err := c.AwaitClusterRequest(ctx, r.ID); err != nil {
			return fmt.Errorf("request %d: %w", r.ID, err)
		}
	}
	return nil
}

func validateVectorSearchUnchanged(d *schema.ResourceDiff) error {
	_, set := castToNestedBlock(d.Get("vector_search"))
	return vectorSearchChangeError(d.Id() != "", d.HasChange("vector_search"), set)
}

// vectorSearchChangeError rejects adding or changing vector_search on an
// existing cluster. Removing the block is allowed and leaves the nodes in place.
func vectorSearchChangeError(exists, changed, set bool) error {
	if exists && changed && set {
		return errVectorSearchChanged
	}
	return nil
}
