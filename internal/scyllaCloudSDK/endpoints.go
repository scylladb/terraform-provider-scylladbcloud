package scyllaCloudSDK

import (
	"errors"
	"fmt"
)

func (c *Client) ListCloudProviders() ([]CloudProvider, error) {
	var result []CloudProvider
	if err := c.get("/deployment/provider", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListCloudProviderRegions(providerId int64) ([]CloudProviderRegion, error) {
	var result []CloudProviderRegion
	path := fmt.Sprintf("/deployment/provider/%d/region", providerId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetCluster(clusterId int64) (Cluster, error) {
	var result Cluster
	path := fmt.Sprintf("/account/%d/cluster/%d/", c.accountId, clusterId)
	err := c.get(path, &result)
	return result, err
}

func (c *Client) ListClusters() ([]Cluster, error) {
	type Item struct {
		Value Cluster     `json:"Value"`
		Error interface{} `json:"Error"`
	}
	var result []Item
	path := fmt.Sprintf("/account/%d/cluster", c.accountId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	clusters := make([]Cluster, len(result))
	for i, item := range result {
		if item.Error != nil {
			return nil, errors.New(fmt.Sprintf("cluster error: %v", item.Error))
		}
		clusters[i] = item.Value
	}
	return clusters, nil
}
