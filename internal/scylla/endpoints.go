package scylla

import (
	"context"
	"fmt"
	"strconv"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func (c *Client) ListCloudProviders(ctx context.Context) ([]model.CloudProvider, error) {
	var result model.CloudProviders
	if err := c.get(ctx, "/deployment/cloud-providers", &result); err != nil {
		return nil, err
	}
	return result.CloudProviders, nil
}

func (c *Client) ListCloudProviderRegions(ctx context.Context, providerID int64) (*model.CloudProviderRegions, error) {
	var result model.CloudProviderRegions
	path := fmt.Sprintf("/deployment/cloud-provider/%d/regions", providerID)
	if err := c.get(ctx, path, &result, "defaults", "true"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListScyllaVersions(ctx context.Context) (*model.ScyllaVersions, error) {
	var result model.ScyllaVersions

	path := "/deployment/scylla-versions"

	if err := c.get(ctx, path, &result, "defaults", "true"); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) ListCloudProviderInstances(ctx context.Context, providerID int64) ([]model.CloudProviderInstance, error) {
	var result model.CloudProviderRegions
	path := fmt.Sprintf("/deployment/cloud-provider/%d/regions", providerID)
	if err := c.get(ctx, path, &result, "defaults", "true"); err != nil {
		return nil, err
	}
	return result.Instances, nil
}

func (c *Client) ListCloudProviderInstancesPerRegion(ctx context.Context, providerID int64, regionID int64) ([]model.CloudProviderInstance, error) {
	var result model.CloudProviderInstances
	path := fmt.Sprintf("/deployment/cloud-provider/%d/region/%d", providerID, regionID)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Instances, nil
}

func (c *Client) GetCluster(ctx context.Context, clusterID int64) (*model.Cluster, error) {
	var result struct {
		Cluster model.Cluster `json:"cluster"`
	}

	path := fmt.Sprintf("/account/%d/cluster/%d", c.AccountID, clusterID)
	err := c.get(ctx, path, &result, "enriched", "true")

	return &result.Cluster, err
}

func (c *Client) Bundle(ctx context.Context, clusterID int64) ([]byte, error) {
	var raw []byte

	path := fmt.Sprintf("/account/%d/cluster/%d/bundle", c.AccountID, clusterID)

	if err := c.get(ctx, path, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}

func (c *Client) Connect(ctx context.Context, clusterID int64) (*model.ClusterConnectionInformation, error) {
	var result model.ClusterConnectionInformation

	path := fmt.Sprintf("/account/%d/cluster/connect", c.AccountID)

	if err := c.get(ctx, path, &result, "clusterId", strconv.FormatInt(clusterID, 10)); err != nil {
		return nil, err
	}

	fix_sf3112(&result) // TODO(rjeczalik): remove when scylladb/siren-frontend#3112 gets fixed

	return &result, nil
}

func (c *Client) CreateCluster(ctx context.Context, req *model.ClusterCreateRequest) (*model.ClusterRequest, error) {
	var result struct {
		RequestID int64 `json:"requestId"`
	}

	path := fmt.Sprintf("/account/%d/cluster", c.AccountID)

	if err := c.post(ctx, path, req, &result); err != nil {
		return nil, err
	}

	var clusterReq model.ClusterRequest

	path = fmt.Sprintf("/account/%d/cluster/request/%d", c.AccountID, result.RequestID)

	if err := c.get(ctx, path, &clusterReq); err != nil {
		return nil, err
	}

	return &clusterReq, nil
}

func (c *Client) DeleteCluster(ctx context.Context, clusterID int64, clusterName string) (*model.ClusterRequest, error) {
	var result model.ClusterRequest

	path := fmt.Sprintf("/account/%d/cluster/%d/delete", c.AccountID, clusterID)
	data := map[string]interface{}{
		"clusterName": clusterName,
	}

	if err := c.post(ctx, path, data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListClusters(ctx context.Context) ([]model.Cluster, error) {
	var result model.Clusters

	path := fmt.Sprintf("/account/%d/clusters", c.AccountID)

	if err := c.get(ctx, path, &result, "enriched", "true"); err != nil {
		return nil, err
	}

	return result.Clusters, nil
}

func (c *Client) ListClusterRequest(ctx context.Context, clusterID int64, typ string) ([]model.ClusterRequest, error) {
	var (
		result []model.ClusterRequest
		query  []string
	)

	if typ != "" {
		query = append(query, "type", typ)
	}

	path := fmt.Sprintf("/account/%d/cluster/%d/request", c.AccountID, clusterID)

	if err := c.get(ctx, path, &result, query...); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetClusterRequest(ctx context.Context, requestID int64) (model.ClusterRequest, error) {
	var result model.ClusterRequest
	path := fmt.Sprintf("/account/%d/cluster/request/%d", c.AccountID, requestID)
	err := c.get(ctx, path, &result)
	return result, err
}

func (c *Client) ListAllowlistRules(ctx context.Context, clusterID int64) ([]model.AllowedIP, error) {
	var result []model.AllowedIP

	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.AccountID, clusterID)

	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CreateAllowlistRule(ctx context.Context, clusterID int64, address string) ([]model.AllowedIP, error) {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed", c.AccountID, clusterID)

	var result []model.AllowedIP

	data := map[string]interface{}{
		"ipAddress": address,
	}

	if err := c.post(ctx, path, data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) DeleteAllowlistRule(ctx context.Context, clusterID, ruleID int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/firewall/allowed/%d", c.AccountID, clusterID, ruleID)

	return c.delete(ctx, path)
}

func (c *Client) ListDataCenters(ctx context.Context, clusterID int64) ([]model.Datacenter, error) {
	var result model.Datacenters

	path := fmt.Sprintf("/account/%d/cluster/%d/dcs", c.AccountID, clusterID)

	if err := c.get(ctx, path, &result, "enriched", "true"); err != nil {
		return nil, err
	}

	return result.Datacenters, nil
}

func (c *Client) ListClusterNodes(ctx context.Context, clusterID int64) ([]model.Node, error) {
	var result model.Nodes

	path := fmt.Sprintf("/account/%d/cluster/%d/nodes", c.AccountID, clusterID)

	if err := c.get(ctx, path, &result, "enriched", "true"); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

func (c *Client) ListClusterVPCPeerings(ctx context.Context, clusterID int64) ([]model.VPCPeering, error) {
	var result []model.VPCPeering

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer", c.AccountID, clusterID)

	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) CreateClusterVPCPeering(ctx context.Context, clusterID int64, req *model.VPCPeeringRequest) (*model.VPCPeering, error) {
	var result struct {
		ID         int64  `json:"id"`
		ExternalID string `json:"externalId"`
	}

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer", c.AccountID, clusterID)

	if err := c.post(ctx, path, req, &result); err != nil {
		return nil, err
	}

	return c.GetClusterVPCPeering(ctx, clusterID, result.ID)
}

func (c *Client) GetClusterVPCPeering(ctx context.Context, clusterID, peerID int64) (*model.VPCPeering, error) {
	var result model.VPCPeering

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer/%d", c.AccountID, clusterID, peerID)

	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) DeleteClusterVPCPeering(ctx context.Context, clusterID, peerID int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/peer/%d", c.AccountID, clusterID, peerID)

	return c.delete(ctx, path)
}

func (c *Client) CreateClusterConnection(ctx context.Context, clusterID int64, req *model.ClusterConnectionCreateRequest) (*model.ClusterConnection, error) {
	var result struct {
		ID           int64 `json:"id"`
		ConnectionID int64 `json:"connectionID"`
	}

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/connection", c.AccountID, clusterID)

	if err := c.post(ctx, path, req, &result); err != nil {
		return nil, err
	}
	return c.GetClusterConnection(ctx, clusterID, result.ConnectionID)
}

func (c *Client) GetClusterConnection(ctx context.Context, clusterID, connectionID int64) (*model.ClusterConnection, error) {
	var result model.ClusterConnection

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/connection/%d", c.AccountID, clusterID, connectionID)

	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListClusterConnections(ctx context.Context, clusterID int64) ([]model.ClusterConnection, error) {
	result := struct {
		Connections []model.ClusterConnection
	}{}

	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/connection", c.AccountID, clusterID)

	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}

	return result.Connections, nil
}

func (c *Client) UpdateClusterConnections(ctx context.Context, clusterID, connectionID int64, req *model.ClusterConnectionUpdateRequest) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/connection/%d", c.AccountID, clusterID, connectionID)
	if err := c.patch(ctx, path, req, nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteClusterConnection(ctx context.Context, clusterID, connectionID int64) error {
	path := fmt.Sprintf("/account/%d/cluster/%d/network/vpc/connection/%d", c.AccountID, clusterID, connectionID)

	return c.delete(ctx, path)
}

func fix_sf3112(c *model.ClusterConnectionInformation) {
	for i := range c.Datacenters {
		dc := &c.Datacenters[i]

		dc.PublicIP = nonempty(dc.PublicIP)
		dc.PrivateIP = nonempty(dc.PrivateIP)
		dc.DNS = nonempty(dc.DNS)
	}
}

func nonempty(s []string) (f []string) {
	for _, s := range s {
		if s != "" {
			f = append(f, s)
		}
	}

	return f
}
