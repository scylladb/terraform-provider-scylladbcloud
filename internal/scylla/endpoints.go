package scylla

import (
	"errors"
	"fmt"
	"github.com/scylladb/terraform-provider-scylla/internal/scylla/model"
)

func (c *Client) ListCloudProviders() ([]model.CloudProvider, error) {
	var result []model.CloudProvider
	if err := c.get("/deployment/provider", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListCloudProviderRegions(providerId int64) ([]model.CloudProviderRegion, error) {
	var result []model.CloudProviderRegion
	path := fmt.Sprintf("/deployment/provider/%d/region", providerId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListClusters() ([]model.Cluster, error) {
	type Item struct {
		Value model.Cluster `json:"Value"`
		Error interface{}   `json:"Error"`
	}
	var result []Item
	path := fmt.Sprintf("/account/%d/cluster", c.accountId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	clusters := make([]model.Cluster, len(result))
	for i, item := range result {
		if item.Error != nil {
			return nil, errors.New(fmt.Sprintf("cluster error: %v", item.Error))
		}
		clusters[i] = item.Value
	}
	return clusters, nil
}

func (c *Client) ListAllowlistRules(clusterId int64) ([]model.AllowlistRule, error) {
	var result []model.AllowlistRule
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.accountId, clusterId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetAllowlistRule(clusterId, ruleId int64) (*model.AllowlistRule, error) {
	var result model.AllowlistRule
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.accountId, clusterId, ruleId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateAllowlistRule(clusterId int64, address string) (*model.AllowlistRule, error) {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.accountId, clusterId)
	var result model.AllowlistRule
	data := map[string]interface{}{
		"CIDRBlock": address,
	}
	if err := c.post(path, data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAllowlistRule(clusterId, ruleId int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.accountId, clusterId, ruleId)
	return c.delete(path)
}

func (c *Client) ListDataCenters(clusterId int64) ([]DataCenter, error) {
	var result []DataCenter
	path := fmt.Sprintf("/account/%d/cluster/%d/dc", c.accountId, clusterId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListClusterNodes(clusterId int64) ([]Node, error) {
	var result []Node
	path := fmt.Sprintf("/account/%d/cluster/%d/node", c.accountId, clusterId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}
