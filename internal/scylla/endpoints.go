package scylla

import (
	"errors"
	"fmt"
	"github.com/scylladb/terraform-provider-scylla/internal/scylla/model"
	"golang.org/x/exp/slices"
)

func (c *Client) ListCloudProviders() ([]model.CloudProvider, error) {
	var result []model.CloudProvider
	if err := c.get("/deployment/provider", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListCloudProviderRegions(providerID int64) ([]model.CloudProviderRegion, error) {
	var result []model.CloudProviderRegion
	path := fmt.Sprintf("/deployment/provider/%d/region", providerID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListCloudProviderInstances(providerID int64) ([]model.CloudProviderInstance, error) {
	var result []model.CloudProviderInstance
	path := fmt.Sprintf("/deployment/provider/%d/instance", providerID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetCluster(clusterID int64) (model.Cluster, error) {
	var result model.Cluster
	path := fmt.Sprintf("/account/%d/cluster/%d/", c.accountID, clusterID)
	err := c.get(path, &result)
	return result, err
}

func (c *Client) CreateCluster(data map[string]interface{}) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	path := fmt.Sprintf("/account/%d/cluster/", c.accountID)

	allowedFields := []string{
		"cidrBlock",
		"name",
		"nodes",
		"cloudProvider",
		"cloudProviderRegion",
		"cloudProviderInstanceType",
		"replicationFactor",
		"enableDnsAssociation",
		"broadcastType",
		"userApiInterface",
	}

	for field, _ := range data {
		if !slices.Contains(allowedFields, field) {
			// This is an error in provider logic, so panic rather than return an error.
			panic(fmt.Sprintf("field %s is not allowed", field))
		}
	}

	if err := c.post(path, data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteCluster(clusterID int64, clusterName string) (*model.ClusterRequest, error) {
	var result model.ClusterRequest
	path := fmt.Sprintf("/account/%d/cluster/%d/delete", c.accountID, clusterID)
	data := map[string]interface{}{
		"clusterName": clusterName,
	}

	if err := c.post(path, data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListClusters() ([]model.Cluster, error) {
	type Item struct {
		Value model.Cluster `json:"Value"`
		Error interface{}   `json:"Error"`
	}
	var result []Item
	path := fmt.Sprintf("/account/%d/cluster", c.accountID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	clusters := make([]model.Cluster, len(result))
	for i, item := range result {
		if item.Error != nil {
			// TODO jmolinski: it's not clear how to handle the case when only some clusters have associated errors
			return nil, errors.New(fmt.Sprintf("cluster error: %v", item.Error))
		}
		clusters[i] = item.Value
	}
	return clusters, nil
}

func (c *Client) GetClusterRequest(requestID int64) (model.ClusterRequest, error) {
	var result model.ClusterRequest
	path := fmt.Sprintf("/account/%d/cluster/request/%d/", c.accountID, requestID)
	err := c.get(path, &result)
	return result, err
}

func (c *Client) ListAllowlistRules(clusterID int64) ([]model.AllowlistRule, error) {
	var result []model.AllowlistRule
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.accountID, clusterID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetAllowlistRule(clusterID, ruleID int64) (*model.AllowlistRule, error) {
	var result model.AllowlistRule
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.accountID, clusterID, ruleID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateAllowlistRule(clusterID int64, address string) (*model.AllowlistRule, error) {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.accountID, clusterID)
	var result model.AllowlistRule
	data := map[string]interface{}{
		"CIDRBlock": address,
	}
	if err := c.post(path, data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAllowlistRule(clusterID, ruleID int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.accountID, clusterID, ruleID)
	return c.delete(path)
}

func (c *Client) ListDataCenters(clusterID int64) ([]model.DataCenter, error) {
	var result []model.DataCenter
	path := fmt.Sprintf("/account/%d/cluster/%d/dc", c.accountID, clusterID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListClusterNodes(clusterID int64) ([]model.Node, error) {
	var result []model.Node
	path := fmt.Sprintf("/account/%d/cluster/%d/node", c.accountID, clusterID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListClusterVPCs(clusterID int64) ([]model.VPC, error) {
	var result []model.VPC
	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc", c.accountID, clusterID)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}
