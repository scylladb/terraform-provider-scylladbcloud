package scylla

import (
	"fmt"
	"strconv"

	"github.com/scylladb/terraform-provider-scylla/internal/scylla/model"
)

func (c *Client) ListCloudProviders() ([]model.CloudProvider, error) {
	var result model.CloudProviders
	if err := c.get("/deployment/cloud-providers", &result); err != nil {
		return nil, err
	}
	return result.CloudProviders, nil
}

func (c *Client) ListCloudProviderRegions(providerID int64) (*model.CloudProviderRegions, error) {
	var result model.CloudProviderRegions
	path := fmt.Sprintf("/deployment/cloud-provider/%d/regions", providerID)
	if err := c.get(path, &result, "defaults", "true"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListScyllaVersions() (*model.ScyllaVersions, error) {
	var result model.ScyllaVersions

	path := "/deployment/scylla-versions"

	if err := c.get(path, &result, "defaults", "true"); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) ListCloudProviderInstances(providerID int64) ([]model.CloudProviderInstance, error) {
	var result model.CloudProviderRegions
	path := fmt.Sprintf("/deployment/cloud-provider/%d/regions", providerID)
	if err := c.get(path, &result, "defaults", "true"); err != nil {
		return nil, err
	}
	return result.Instances, nil
}

func (c *Client) GetCluster(clusterID int64) (*model.Cluster, error) {
	var result struct {
		Cluster model.Cluster `json:"cluster"`
	}

	path := fmt.Sprintf("/account/%d/cluster/%d", c.AccountID, clusterID)
	err := c.get(path, &result, "enriched", "true")

	return &result.Cluster, err
}

func (c *Client) Connect(clusterID int64) (*model.ClusterConnection, error) {
	var result model.ClusterConnection

	path := fmt.Sprintf("/account/%d/cluster/connect", c.AccountID)

	if err := c.get(path, &result, "clusterId", strconv.FormatInt(clusterID, 10)); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) CreateCluster(req *model.ClusterCreateRequest) (*model.ClusterRequest, error) {
	var result struct {
		RequestID int64 `json:"requestId"`
	}

	path := fmt.Sprintf("/account/%d/cluster", c.AccountID)

	if err := c.post(path, req, &result); err != nil {
		return nil, err
	}

	var clusterReq model.ClusterRequest

	path = fmt.Sprintf("/account/%d/cluster/request/%d", c.AccountID, result.RequestID)

	if err := c.get(path, &clusterReq); err != nil {
		return nil, err
	}

	return &clusterReq, nil
}

func (c *Client) DeleteCluster(clusterID int64, clusterName string) (*model.ClusterRequest, error) {
	var result model.ClusterRequest

	path := fmt.Sprintf("/account/%d/cluster/%d/delete", c.AccountID, clusterID)
	data := map[string]interface{}{
		"clusterName": clusterName,
	}

	if err := c.post(path, data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListClusters() ([]model.Cluster, error) {
	var result model.Clusters

	path := fmt.Sprintf("/account/%d/clusters", c.AccountID)

	if err := c.get(path, &result, "enriched", "true"); err != nil {
		return nil, err
	}

	return result.Clusters, nil
}

func (c *Client) ListClusterRequest(clusterID int64, typ string) ([]model.ClusterRequest, error) {
	var (
		result []model.ClusterRequest
		query  []string
	)

	if typ != "" {
		query = append(query, "type", typ)
	}

	path := fmt.Sprintf("/account/%d/cluster/%d/request", c.AccountID, clusterID)

	if err := c.get(path, &result, query...); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetClusterRequest(requestID int64) (model.ClusterRequest, error) {
	var result model.ClusterRequest
	path := fmt.Sprintf("/account/%d/cluster/request/%d", c.AccountID, requestID)
	err := c.get(path, &result)
	return result, err
}

func (c *Client) ListAllowlistRules(clusterID int64) ([]model.AllowedIP, error) {
	var result []model.AllowedIP

	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.AccountID, clusterID)

	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CreateAllowlistRule(clusterID int64, address string) ([]model.AllowedIP, error) {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.AccountID, clusterID)

	var result []model.AllowedIP

	data := map[string]interface{}{
		"ipAddress": address,
	}

	if err := c.post(path, data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) DeleteAllowlistRule(clusterID, ruleID int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.AccountID, clusterID, ruleID)

	return c.delete(path)
}

func (c *Client) ListDataCenters(clusterID int64) ([]model.Datacenter, error) {
	var result model.Datacenters

	path := fmt.Sprintf("/account/%d/cluster/%d/dcs", c.AccountID, clusterID)

	if err := c.get(path, &result, "enriched", "true"); err != nil {
		return nil, err
	}

	return result.Datacenters, nil
}

func (c *Client) ListClusterNodes(clusterID int64) ([]model.Node, error) {
	var result model.Nodes

	path := fmt.Sprintf("/account/%d/cluster/%d/nodes", c.AccountID, clusterID)

	if err := c.get(path, &result, "enriched", "true"); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

func (c *Client) ListClusterVPCPeerings(clusterID int64) ([]model.VPCPeering, error) {
	var result []model.VPCPeering

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer", c.AccountID, clusterID)

	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) CreateClusterVPCPeering(clusterID int64, req *model.VPCPeeringRequest) (*model.VPCPeering, error) {
	var result struct {
		ID         int64  `json:"id"`
		ExternalID string `json:"externalId"`
	}

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer", c.AccountID, clusterID)

	if err := c.post(path, req, &result); err != nil {
		return nil, err
	}

	return c.GetClusterVPCPeering(clusterID, result.ID)
}

func (c *Client) GetClusterVPCPeering(clusterID, peerID int64) (*model.VPCPeering, error) {
	var result model.VPCPeering

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer/%d", c.AccountID, clusterID, peerID)

	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) DeleteClusterVPCPeering(clusterID, peerID int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer/%d", c.AccountID, clusterID, peerID)

	return c.delete(path)
}
