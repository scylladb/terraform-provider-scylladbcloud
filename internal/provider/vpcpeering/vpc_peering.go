package vpcpeering

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/schemautils"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	vpcPeeringRetryTimeout    = 40 * time.Minute
	vpcPeeringDeleteTimeout   = 90 * time.Minute
	vpcPeeringRetryDelay      = 5 * time.Second
	vpcPeeringRetryMinTimeout = 3 * time.Second
)

func ResourceVPCPeering() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVPCPeeringCreate,
		ReadContext:   resourceVPCPeeringRead,
		UpdateContext: resourceVPCPeeringUpdate,
		DeleteContext: resourceVPCPeeringDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(vpcPeeringRetryTimeout),
			Update: schema.DefaultTimeout(vpcPeeringRetryTimeout),
			Delete: schema.DefaultTimeout(vpcPeeringDeleteTimeout),
		},

		SchemaVersion: 1,

		StateUpgraders: []schema.StateUpgrader{
			{
				Version: 0,
				Type:    resourceVPCPeeringV0().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceVPCPeeringUpgradeV0,
			},
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID",
				Required:    true,
				Type:        schema.TypeInt,
			},
			"datacenter": {
				Description: "Cluster datacenter name",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"peer_vpc_id": {
				Description: "Peer VPC ID",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"peer_cidr_blocks": {
				Description: "Peer VPC CIDR block list",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"peer_region": {
				Description: "Peer VPC region",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"peer_account_id": {
				Description: "Peer Account ID",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"allow_cql": {
				Description: "Whether to allow CQL traffic",
				Optional:    true,
				Type:        schema.TypeBool,
				// NOTE(rjeczalik): ForceNew is commented out here, otherwise
				// internal provider validate fails due to all the attrs
				// being ForceNew; Scylla Cloud API does not allow for
				// updating existing vpc peerings, thus the update implementation
				// always returns a non-nil error.
				// ForceNew:    true,
				Default: true,
			},
			"vpc_peering_id": {
				Description: "Cluster VPC Peering ID",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"connection_id": {
				Description: "VPC peering connection id",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"network_link": {
				Description: "(GCP) Cluster VPC network self_link",
				Computed:    true,
				Type:        schema.TypeString,
			},
		},
	}
}

func resourceVPCPeeringCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c                        = meta.(*scylla.Client)
		pr                       = d.Get("peer_region").(string)
		dcName                   = d.Get("datacenter").(string)
		cidrBlocks, cidrBlocksOK = d.GetOk("peer_cidr_blocks")
		r                        = &model.VPCPeeringRequest{
			AllowCQL: d.Get("allow_cql").(bool),
			VPC:      d.Get("peer_vpc_id").(string),
			Owner:    d.Get("peer_account_id").(string),
		}
		clusterID = d.Get("cluster_id").(int)
		p         *scylla.CloudProvider
		dc        *model.Datacenter
	)

	dcs, err := c.ListDataCenters(ctx, int64(clusterID))
	if err != nil {
		return diag.Errorf("error reading clusters: %s", err)
	}

	for i := range dcs {
		dc = &dcs[i]

		if strings.EqualFold(dc.Name, dcName) {
			r.DatacenterID = dc.ID
			p = c.Meta.ProviderByID(dc.CloudProviderID)
			break
		}
	}

	if dc == nil {
		return diag.Errorf("unable to find %q datacenter", dcName)
	}

	if p == nil {
		return diag.Errorf("unable to find cloud provider with id=%d", dc.CloudProviderID)
	}

	region := p.RegionByName(pr)
	if region == nil {
		return diag.Errorf("unrecognized region %q", pr)
	}

	r.RegionID = region.ID
	if !cidrBlocksOK {
		if !strings.EqualFold(p.CloudProvider.Name, "GCP") {
			return diag.Errorf(`"peer_cidr_blocks" is required for %q cloud`, p.CloudProvider.Name)
		}

		cidr, ok := c.Meta.GCPBlocks[pr]
		if !ok {
			return diag.Errorf("no default peer CIDR block found for %q region", pr)
		}

		cidrBlocks = []any{cidr}
	} else if strings.EqualFold(p.CloudProvider.Name, "GCP") {
		cidr, ok := c.Meta.GCPBlocks[pr]
		if !ok {
			return diag.Errorf("no default peer CIDR block found for %q region", pr)
		}

		if blocks := cidrBlocks.([]any); len(blocks) > 1 {
			if blocks[0].(string) == cidr {
				return diag.Errorf(`omit "peer_cidr_blocks" attribute for default GCP cidr blocks`)
			}
		}
	}

	if len(cidrBlocks.([]any)) == 0 {
		return diag.Errorf(`"peer_cidr_blocks" cannot be empty`)
	}

	cidrList, err := schemautils.ConvertListToConcrete[string](cidrBlocks)
	if err != nil {
		return diag.Errorf(`"peer_cidr_blocks" must be a list of strings`)
	}

	// TODO: Use appropriate API field type that supports multiple CIDR blocks
	// once that is done done there's no need to join the CIDR blocks into a string.
	r.CidrBlock = strings.Join(cidrList, ",")

	if r.DatacenterID == 0 {
		return diag.Errorf("unrecognized datacenter %q", dcName)
	}

	vp, err := c.CreateClusterVPCPeering(ctx, int64(clusterID), r)
	if err != nil {
		return diag.Errorf("error creating vpc peering: %s", err)
	}

	d.SetId(vp.ExternalID)

	_ = d.Set("vpc_peering_id", vp.ID)
	_ = d.Set("connection_id", vp.ExternalID)
	_ = d.Set("network_link", vp.NetworkLink())

	return nil
}

func resourceVPCPeeringRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c      = meta.(*scylla.Client)
		connID = d.Id()
		p      *scylla.CloudProvider

		// stateClusterID is 0 on import.
		stateClusterID = int64(d.Get("cluster_id").(int))
	)

	clusters, unreadable, err := c.ListClusters(ctx)
	if err != nil {
		return diag.Errorf("error reading cluster list: %s", err)
	}

	// All the clusters of a shared network list its peerings, so a refresh
	// looks in the cluster of the state first. unchecked counts the clusters
	// that may list the peering but cannot be read.
	var (
		stateCluster, others []model.ClusterSummary
		unchecked            = unreadable
	)
	for _, cl := range clusters {
		switch {
		case cl.ID == stateClusterID:
			stateCluster = append(stateCluster, cl)
		case cl.DCCount == 0:
			// The API counts only ACTIVE datacenters, and reading a cluster
			// without one fails. A cluster whose create failed early never
			// gets one.
			unchecked++
		default:
			others = append(others, cl)
		}
	}

	// The cluster of the peering can be listed without an ACTIVE datacenter
	// too. Removing the peering from the state is safe only when its cluster
	// was read or is gone.
	if len(stateCluster) > 0 && stateCluster[0].DCCount == 0 {
		return retryDiag(stateClusterID, "cluster %d of VPC peering %q is listed without a datacenter, so the peering cannot be read: "+
			"try again when the cluster has an ACTIVE datacenter", stateClusterID, connID)
	}

	cluster, vpcPeering, err := findPeering(ctx, c, stateCluster, connID)
	if err == nil && cluster == nil {
		cluster, vpcPeering, err = findPeering(ctx, c, others, connID)
	}
	if err != nil {
		return diag.FromErr(err)
	}

	if cluster == nil && unchecked > 0 {
		return retryDiag(stateClusterID, "VPC peering %q was not found, but %d of the clusters of the account could not be read, "+
			"and they may list it: try again later", connID, unchecked)
	}

	if cluster == nil {
		// The peering or its cluster was deleted.
		d.SetId("")
		return nil
	}

	// Taking the peering from another cluster would change cluster_id, which
	// plans an update that always fails, and removing it from the state would
	// leave it in the cloud.
	if stateClusterID != 0 && cluster.ID != stateClusterID {
		// The list has no ID for a cluster the API could not read, so such a
		// cluster can be the cluster of the state, and still list the peering.
		if len(stateCluster) == 0 && unreadable > 0 {
			return retryDiag(stateClusterID, "VPC peering %q is listed by cluster %d, but cluster %d of the state is not listed, "+
				"and the API could not read %d of the clusters of the account: try again later", connID, cluster.ID, stateClusterID, unreadable)
		}

		return diag.Diagnostics{{
			Severity: diag.Error,
			Summary: fmt.Sprintf("VPC peering %q is not listed by cluster %d of the state but by cluster %d: "+
				"to manage it through that cluster, remove it from the state and import it again", connID, stateClusterID, cluster.ID),
			Detail: "An import takes the first cluster that lists the peering. Check the `cluster_id` that it stores, " +
				"and the next plan, before applying.",
		}}
	}

	if p = c.Meta.ProviderByID(cluster.CloudProviderID); p == nil {
		return diag.Errorf("unable to find cloud provider with id=%d", cluster.CloudProviderID)
	}

	// RegionID is always 0 for GCP peering
	if vpcPeering.RegionID != 0 {
		if r := p.RegionByID(vpcPeering.RegionID); r != nil {
			_ = d.Set("peer_region", r.ExternalID)
		}
	}

	_ = d.Set("datacenter", cluster.Datacenter.Name)
	_ = d.Set("peer_vpc_id", vpcPeering.VPCID)
	_ = d.Set("peer_account_id", vpcPeering.OwnerID)
	_ = d.Set("vpc_peering_id", vpcPeering.ID)
	_ = d.Set("connection_id", vpcPeering.ExternalID)
	_ = d.Set("cluster_id", cluster.ID)
	_ = d.Set("network_link", vpcPeering.NetworkLink())
	_ = d.Set("allow_cql", vpcPeering.AllowCQL)
	_ = d.Set("peer_cidr_blocks", vpcPeering.CIDRList)

	return nil
}

// findPeering returns the first of clusters that lists the peering connID, and
// that peering, or nil when none does.
func findPeering(ctx context.Context, c *scylla.Client, clusters []model.ClusterSummary, connID string) (*model.Cluster, *model.VPCPeering, error) {
	for i := range clusters {
		cluster, err := c.GetCluster(ctx, clusters[i].ID)
		if err != nil {
			return nil, nil, fmt.Errorf("error reading cluster ID=%d: %w", clusters[i].ID, err)
		}

		for j := range cluster.VPCPeeringList {
			if strings.EqualFold(cluster.VPCPeeringList[j].ExternalID, connID) {
				return cluster, &cluster.VPCPeeringList[j], nil
			}
		}
	}

	return nil, nil, nil
}

// retryDiag returns an error that keeps the peering in the state. On a
// refresh, its detail names the way out when trying again does not help.
// stateClusterID is 0 on import.
func retryDiag(stateClusterID int64, format string, a ...any) diag.Diagnostics {
	d := diag.Diagnostic{
		Severity: diag.Error,
		Summary:  fmt.Sprintf(format, a...),
	}

	if stateClusterID != 0 {
		d.Detail = "If the peering was deleted, or Terraform should stop managing it, `terraform state rm` " +
			"removes it from the state. A peering that still exists stays in place."
	}

	return diag.Diagnostics{d}
}

func resourceVPCPeeringUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.Errorf(`updating "scylla_vpc_peering" resource is not supported`)
}

func resourceVPCPeeringDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	peerID, ok := d.GetOk("vpc_peering_id")
	if !ok {
		return diag.Errorf("unable to read VPC peering ID from state file")
	}

	clusterID, ok := d.GetOk("cluster_id")
	if !ok {
		return diag.Errorf("unable to read cluster ID from state file")
	}

	if err := c.DeleteClusterVPCPeering(ctx, int64(clusterID.(int)), int64(peerID.(int))); err != nil {
		if scylla.IsDeletedErr(err) {
			return nil // cluster was already deleted
		}

		return diag.Errorf("error deleting vpc peering: %s", err)
	}

	return nil
}

func resourceVPCPeeringUpgradeV0(ctx context.Context, rawState map[string]interface{}, meta interface{}) (map[string]interface{}, error) {
	if cidr, ok := rawState["peer_cidr_block"].(string); ok {
		rawState["peer_cidr_blocks"] = split(cidr, ",")
		delete(rawState, "peer_cidr_block")
	}

	return rawState, nil
}

func split(s, sep string) (x []any) {
	for _, v := range strings.Split(s, sep) {
		if v := strings.TrimSpace(v); v != "" {
			x = append(x, v)
		}
	}
	return x
}
