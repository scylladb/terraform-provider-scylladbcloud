package datasource

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func clusterIDArg() *schema.Schema {
	return &schema.Schema{Type: schema.TypeInt, Required: true, Description: "Cluster ID."}
}

func clusterSummary(c *model.Cluster) map[string]interface{} {
	m := map[string]interface{}{
		"cluster_id": c.ID,
		"name":       c.ClusterName,
		"status":     c.Status,
		"region":     "",
		"cloud":      "",
	}
	if c.Region != nil {
		m["region"] = c.Region.ExternalID
	}
	if c.CloudProvider != nil {
		m["cloud"] = c.CloudProvider.Name
	}
	return m
}

func clusterSummarySchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"cluster_id": computed(schema.TypeInt, "Cluster ID."),
		"name":       computed(schema.TypeString, "Cluster name."),
		"status":     computed(schema.TypeString, "Cluster status, e.g. ACTIVE."),
		"cloud":      computed(schema.TypeString, "Cloud provider, e.g. AWS."),
		"region":     computed(schema.TypeString, "Cloud region, e.g. us-east-1."),
	}
}

func DataSourceClusters() *schema.Resource {
	return &schema.Resource{
		Description: "The account's clusters, deleted ones excluded.",
		ReadContext: dataSourceClustersRead,
		Schema: map[string]*schema.Schema{
			"clusters": computedList("Clusters.", clusterSummarySchema()),
		},
	}
}

func dataSourceClustersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return diag.Errorf("failed to list clusters: %s", err)
	}
	out := make([]map[string]interface{}, 0, len(clusters))
	for i := range clusters {
		if strings.EqualFold(clusters[i].Status, "DELETED") {
			continue
		}
		out = append(out, clusterSummary(&clusters[i]))
	}
	d.SetId(strconv.FormatInt(c.AccountID, 10))
	_ = d.Set("clusters", out)
	return nil
}

func DataSourceCluster() *schema.Resource {
	s := clusterSummarySchema()
	s["cluster_id"] = clusterIDArg()
	s["datacenter"] = computed(schema.TypeString, "Datacenter name.")
	s["node_count"] = computed(schema.TypeInt, "Number of ACTIVE nodes.")
	s["scylla_version"] = computed(schema.TypeString, "Scylla version.")
	s["replication_factor"] = computed(schema.TypeInt, "Replication factor.")
	s["client_encryption_mode"] = computed(schema.TypeString, "Client-to-node encryption mode.")
	s["user_api_interface"] = computed(schema.TypeString, "CQL or ALTERNATOR.")
	s["dns"] = computed(schema.TypeBool, "Whether DNS names are enabled.")
	s["node_private_ips"] = stringList("Private IPs of the ACTIVE nodes.")
	s["node_dns_names"] = stringList("DNS names of the ACTIVE nodes.")
	return &schema.Resource{
		Description: "One cluster, by ID.",
		ReadContext: dataSourceClusterRead,
		Schema:      s,
	}
}

func dataSourceClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	id := int64(d.Get("cluster_id").(int))
	cluster, err := c.GetCluster(ctx, id)
	if err != nil {
		return diag.Errorf("failed to read cluster %d: %s", id, err)
	}
	for k, v := range clusterSummary(cluster) {
		_ = d.Set(k, v)
	}
	active := model.NodesByStatus(cluster.Nodes, "ACTIVE")
	d.SetId(strconv.FormatInt(id, 10))
	_ = d.Set("node_count", len(active))
	_ = d.Set("node_private_ips", model.NodesPrivateIPs(active))
	_ = d.Set("node_dns_names", model.NodesDNSNames(active))
	_ = d.Set("replication_factor", cluster.ReplicationFactor)
	_ = d.Set("client_encryption_mode", cluster.EncryptionMode)
	_ = d.Set("user_api_interface", cluster.UserAPIInterface)
	_ = d.Set("dns", cluster.DNS)
	if cluster.Datacenter != nil {
		_ = d.Set("datacenter", cluster.Datacenter.Name)
	}
	if cluster.ScyllaVersion != nil {
		_ = d.Set("scylla_version", cluster.ScyllaVersion.Version)
	}
	return nil
}

func DataSourceClusterNodes() *schema.Resource {
	return &schema.Resource{
		Description: "The nodes of a cluster.",
		ReadContext: dataSourceClusterNodesRead,
		Schema: map[string]*schema.Schema{
			"cluster_id": clusterIDArg(),
			"nodes": computedList("Nodes.", map[string]*schema.Schema{
				"node_id":       computed(schema.TypeInt, "Node ID."),
				"status":        computed(schema.TypeString, "Node status, e.g. ACTIVE."),
				"state":         computed(schema.TypeString, "Node state."),
				"private_ip":    computed(schema.TypeString, "Private IP."),
				"public_ip":     computed(schema.TypeString, "Public IP."),
				"dns":           computed(schema.TypeString, "DNS name."),
				"datacenter_id": computed(schema.TypeInt, "Datacenter ID."),
				"instance_id":   computed(schema.TypeInt, "Instance type ID."),
			}),
		},
	}
}

func dataSourceClusterNodesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := int64(d.Get("cluster_id").(int))
	nodes, err := meta.(*scylla.Client).ListClusterNodes(ctx, id)
	if err != nil {
		return diag.Errorf("failed to list nodes of cluster %d: %s", id, err)
	}
	out := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, map[string]interface{}{
			"node_id": n.ID, "status": n.Status, "state": n.State, "private_ip": n.PrivateIP,
			"public_ip": n.PublicIP, "dns": n.DNS, "datacenter_id": n.DatacenterID, "instance_id": n.InstanceID,
		})
	}
	d.SetId(strconv.FormatInt(id, 10))
	_ = d.Set("nodes", out)
	return nil
}

func DataSourceClusterDatacenters() *schema.Resource {
	return &schema.Resource{
		Description: "The datacenters of a cluster.",
		ReadContext: dataSourceClusterDatacentersRead,
		Schema: map[string]*schema.Schema{
			"cluster_id": clusterIDArg(),
			"datacenters": computedList("Datacenters.", map[string]*schema.Schema{
				"datacenter_id":      computed(schema.TypeInt, "Datacenter ID."),
				"name":               computed(schema.TypeString, "Datacenter name."),
				"status":             computed(schema.TypeString, "Datacenter status."),
				"cloud_provider_id":  computed(schema.TypeInt, "Cloud provider ID."),
				"region_id":          computed(schema.TypeInt, "Region ID."),
				"cidr_block":         computed(schema.TypeString, "CIDR block."),
				"replication_factor": computed(schema.TypeInt, "Replication factor."),
			}),
		},
	}
}

func dataSourceClusterDatacentersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := int64(d.Get("cluster_id").(int))
	dcs, err := meta.(*scylla.Client).ListDataCenters(ctx, id)
	if err != nil {
		return diag.Errorf("failed to list datacenters of cluster %d: %s", id, err)
	}
	out := make([]map[string]interface{}, 0, len(dcs))
	for _, dc := range dcs {
		out = append(out, map[string]interface{}{
			"datacenter_id": dc.ID, "name": dc.Name, "status": dc.Status, "cloud_provider_id": dc.CloudProviderID,
			"region_id": dc.RegionID, "cidr_block": dc.CIDRBlock, "replication_factor": dc.ReplicationFactor,
		})
	}
	d.SetId(strconv.FormatInt(id, 10))
	_ = d.Set("datacenters", out)
	return nil
}
