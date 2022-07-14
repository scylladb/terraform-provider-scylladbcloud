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

func (c *Client) ListAllowlistRules(clusterId int64) ([]AllowlistRule, error) {
	var result []AllowlistRule
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.accountId, clusterId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetAllowlistRule(clusterId, ruleId int64) (*AllowlistRule, error) {
	var result AllowlistRule
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.accountId, clusterId, ruleId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateAllowlistRule(clusterId int64, address string) (*AllowlistRule, error) {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.accountId, clusterId)
	var result AllowlistRule
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
