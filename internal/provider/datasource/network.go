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

func DataSourceVPCPeerings() *schema.Resource {
	return &schema.Resource{
		Description: "The VPC peerings of a cluster.",
		ReadContext: dataSourceVPCPeeringsRead,
		Schema: map[string]*schema.Schema{
			"cluster_id": clusterIDArg(),
			"vpc_peerings": computedList("VPC peerings.", map[string]*schema.Schema{
				"vpc_peering_id":   computed(schema.TypeInt, "VPC peering ID."),
				"connection_id":    computed(schema.TypeString, "Cloud peering connection ID."),
				"peer_vpc_id":      computed(schema.TypeString, "Peer VPC ID."),
				"peer_account_id":  computed(schema.TypeString, "Peer account ID."),
				"peer_region_id":   computed(schema.TypeInt, "Peer region ID (0 for GCP)."),
				"peer_cidr_blocks": stringList("Peer CIDR blocks."),
				"status":           computed(schema.TypeString, "Peering status."),
				"allow_cql":        computed(schema.TypeBool, "Whether CQL traffic is allowed."),
			}),
		},
	}
}

func dataSourceVPCPeeringsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := int64(d.Get("cluster_id").(int))
	c := meta.(*scylla.Client)
	peering, err := peeringEnabled(ctx, c, id)
	if err != nil {
		return diag.Errorf("failed to read cluster %d: %s", id, err)
	}
	var peerings []model.VPCPeering
	if peering {
		if peerings, err = c.ListClusterVPCPeerings(ctx, id); err != nil {
			return diag.Errorf("failed to list VPC peerings of cluster %d: %s", id, err)
		}
	}
	out := make([]map[string]interface{}, 0, len(peerings))
	for _, p := range peerings {
		out = append(out, map[string]interface{}{
			"vpc_peering_id": p.ID, "connection_id": p.ExternalID, "peer_vpc_id": p.VPCID, "peer_account_id": p.OwnerID,
			"peer_region_id": p.RegionID, "peer_cidr_blocks": p.CIDRList, "status": p.Status, "allow_cql": p.AllowCQL,
		})
	}
	d.SetId(strconv.FormatInt(id, 10))
	_ = d.Set("vpc_peerings", out)
	return nil
}

func DataSourceClusterConnections() *schema.Resource {
	return &schema.Resource{
		Description: "The connections (transit gateway attachments, peerings, private links) of a cluster.",
		ReadContext: dataSourceClusterConnectionsRead,
		Schema: map[string]*schema.Schema{
			"cluster_id": clusterIDArg(),
			"connections": computedList("Connections.", map[string]*schema.Schema{
				"connection_id": computed(schema.TypeInt, "Connection ID."),
				"name":          computed(schema.TypeString, "Connection name."),
				"type":          computed(schema.TypeString, "Connection type, e.g. AWS_TGW_ATTACHMENT."),
				"status":        computed(schema.TypeString, "Connection status."),
				"external_id":   computed(schema.TypeString, "ID of the cloud resource."),
				"datacenter_id": computed(schema.TypeInt, "Datacenter ID."),
				"cidrlist":      stringList("CIDRs routed to the connection."),
				"data": {
					Type: schema.TypeMap, Computed: true, Description: "Connection data.",
					Elem: &schema.Schema{Type: schema.TypeString},
				},
			}),
		},
	}
}

func dataSourceClusterConnectionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := int64(d.Get("cluster_id").(int))
	c := meta.(*scylla.Client)
	peering, err := peeringEnabled(ctx, c, id)
	if err != nil {
		return diag.Errorf("failed to read cluster %d: %s", id, err)
	}
	var conns []model.ClusterConnection
	if peering {
		if conns, err = c.ListClusterConnections(ctx, id); err != nil {
			return diag.Errorf("failed to list connections of cluster %d: %s", id, err)
		}
	}
	out := make([]map[string]interface{}, 0, len(conns))
	for _, c := range conns {
		out = append(out, map[string]interface{}{
			"connection_id": c.ID, "name": c.Name, "type": c.Type, "status": c.Status, "external_id": c.ExternalID,
			"datacenter_id": c.ClusterDCID, "cidrlist": c.CIDRList, "data": c.Data,
		})
	}
	d.SetId(strconv.FormatInt(id, 10))
	_ = d.Set("connections", out)
	return nil
}

// peeringEnabled reports whether the cluster has VPC peering enabled; the
// API rejects peering and connection listing for public clusters.
func peeringEnabled(ctx context.Context, c *scylla.Client, clusterID int64) (bool, error) {
	cluster, err := c.GetClusterSummary(ctx, clusterID)
	if err != nil {
		return false, err
	}
	return !strings.EqualFold(cluster.BroadcastType, "PUBLIC"), nil
}

func DataSourceCloudAccounts() *schema.Resource {
	return &schema.Resource{
		Description: "The cloud accounts clusters can run in: ScyllaDB's own and any bring-your-own accounts.",
		ReadContext: dataSourceCloudAccountsRead,
		Schema: map[string]*schema.Schema{
			"cloud_accounts": computedList("Cloud accounts.", map[string]*schema.Schema{
				"cloud_account_id":  computed(schema.TypeInt, "Cloud account ID."),
				"cloud_provider_id": computed(schema.TypeInt, "Cloud provider ID."),
				"owner":             computed(schema.TypeString, "Scylla for ScyllaDB's account, else the customer."),
				"state":             computed(schema.TypeString, "Account state, e.g. ACTIVE."),
			}),
		},
	}
}

func dataSourceCloudAccountsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	accounts, err := c.ListCloudAccounts(ctx)
	if err != nil {
		return diag.Errorf("failed to list cloud accounts: %s", err)
	}
	out := make([]map[string]interface{}, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, map[string]interface{}{
			"cloud_account_id": a.ID, "cloud_provider_id": a.CloudProviderID, "owner": a.Owner, "state": a.State,
		})
	}
	d.SetId(strconv.FormatInt(c.AccountID, 10))
	_ = d.Set("cloud_accounts", out)
	return nil
}
