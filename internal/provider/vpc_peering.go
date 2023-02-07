package provider

import (
	"context"
	"strings"
	"time"

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
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(vpcPeeringRetryTimeout),
			Update: schema.DefaultTimeout(vpcPeeringRetryTimeout),
			Delete: schema.DefaultTimeout(vpcPeeringDeleteTimeout),
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
			"peer_cidr_block": {
				Description: "Peer VPC CIDR block",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
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
		},
	}
}

func resourceVPCPeeringCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c      = meta.(*scylla.Client)
		pr     = d.Get("peer_region").(string)
		dcName = d.Get("datacenter").(string)
		r      = &model.VPCPeeringRequest{
			AllowCQL:  d.Get("allow_cql").(bool),
			VPC:       d.Get("peer_vpc_id").(string),
			CidrBlock: d.Get("peer_cidr_block").(string),
			Owner:     d.Get("peer_account_id").(string),
		}
		clusterID = d.Get("cluster_id").(int)
		p         *scylla.CloudProvider
		dc        *model.Datacenter
	)

	dcs, err := c.ListDataCenters(ctx, int64(clusterID))
	if err != nil {
		return diag.Errorf("error reading clusters: %w", err)
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

	if r.DatacenterID == 0 {
		return diag.Errorf("unrecognized datacenter %q", dcName)
	}

	vp, err := c.CreateClusterVPCPeering(ctx, int64(clusterID), r)
	if err != nil {
		return diag.Errorf("error creating vpc peering: %w", err)
	}

	d.SetId(vp.ExternalID)
	d.Set("vpc_peering_id", vp.ID)
	d.Set("connection_id", vp.ExternalID)

	return nil
}

func resourceVPCPeeringRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c          = meta.(*scylla.Client)
		connID     = d.Id()
		cluster    *model.Cluster
		vpcPeering *model.VPCPeering
		p          *scylla.CloudProvider
	)

	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return diag.Errorf("error reading cluster list: %w", err)
	}

lookup:
	for i := range clusters {
		c, err := c.GetCluster(ctx, clusters[i].ID)
		if err != nil {
			return diag.Errorf("error reading cluster ID=%d: %w", clusters[i].ID, err)
		}

		for j := range c.VPCPeeringList {
			vp := &c.VPCPeeringList[j]

			if strings.EqualFold(vp.ExternalID, connID) {
				cluster = c
				vpcPeering = vp
				break lookup
			}
		}
	}

	if cluster == nil {
		return diag.Errorf("unable to find cluster for peering connection ID: %q", connID)
	}

	if vpcPeering == nil {
		return diag.Errorf("unrecognized vpc peering connection ID: %q", connID)
	}

	if p = c.Meta.ProviderByID(cluster.CloudProviderID); p == nil {
		return diag.Errorf("unable to find cloud provider with id=%d", cluster.CloudProviderID)
	}

	d.Set("datacenter", cluster.Datacenter.Name)
	d.Set("peer_vpc_id", vpcPeering.VPCID)
	d.Set("peer_cidr_block", vpcPeering.CIDRList[0])
	d.Set("peer_region", p.RegionByID(vpcPeering.RegionID).ExternalID)
	d.Set("peer_account_id", vpcPeering.OwnerID)
	d.Set("vpc_peering_id", vpcPeering.ID)
	d.Set("connection_id", vpcPeering.ExternalID)
	d.Set("cluster_id", cluster.ID)

	return nil
}

func resourceVPCPeeringUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.Errorf(`updating "scylla_vpc_peering" resource is not supported`)
}

func resourceVPCPeeringDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c = meta.(*scylla.Client)
	)

	peerID, ok := d.GetOk("vpc_peering_id")
	if !ok {
		return diag.Errorf("unable to read VPC peering ID from state file")
	}

	clusterID, ok := d.GetOk("cluster_id")
	if !ok {
		return diag.Errorf("unable to read cluster ID from state file")
	}

	if err := c.DeleteClusterVPCPeering(ctx, int64(clusterID.(int)), int64(peerID.(int))); err != nil {
		return diag.Errorf("error deleting vpc peering: %w", err)
	}

	return nil
}
