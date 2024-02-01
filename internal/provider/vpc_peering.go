package provider

import (
	"context"
	"fmt"
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
			StateContext: schema.ImportStatePassthroughContext,
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
				Description: "Peer VPC CIDR block, this attribute also supports comma separated list of CIDR blocks, but this is deprecated and will be removed in the future versions of the provider, for multiple CIDR blocks use peer_cidr_blocks attribute instead",
				Optional:    true,
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
		cidr, cidrOK             = d.GetOk("peer_cidr_block")
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
	if cidrOK && cidrBlocksOK {
		return diag.Errorf(`"peer_cidr_block" and "peer_cidr_blocks" are mutually exclusive, please specify only one of them`)
	}

	if !cidrOK && !cidrBlocksOK {
		if !strings.EqualFold(p.CloudProvider.Name, "GCP") {
			return diag.Errorf(`"peer_cidr_block" is required for %q cloud`, p.CloudProvider.Name)
		}

		var ok bool
		if cidr, ok = c.Meta.GCPBlocks[pr]; !ok {
			return diag.Errorf("no default peer CIDR block found for %q region", pr)
		}
	} else if strings.EqualFold(p.CloudProvider.Name, "GCP") {
		if c.Meta.GCPBlocks[pr] == cidr.(string) {
			return diag.Errorf(`omit "peer_cidr_block" attribute for default GCP cidr blocks`)
		}
	}

	if cidrBlocksOK {
		if len(cidrBlocks.([]any)) == 0 {
			return diag.Errorf(`"peer_cidr_blocks" cannot be empty`)
		}

		cidrList, err := CIDRList(cidrBlocks)
		if err != nil {
			return diag.Errorf(`"peer_cidr_blocks" must be a list of strings`)
		}

		// TODO: Use appropriate API field type that supports multiple CIDR blocks
		// once that is done done there's no need to join the CIDR blocks into a string.
		r.CidrBlock = strings.Join(cidrList, ",")
	} else {
		r.CidrBlock = cidr.(string)
	}

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

// CIDRList converts a list of CIDR blocks to a comma-separated string.
func CIDRList(cidrBlocks any) ([]string, error) {
	cidrBlocksSlice, ok := cidrBlocks.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", cidrBlocks)
	}

	var cidrList []string
	for _, cidrBlock := range cidrBlocksSlice {
		cidrBlockString, ok := cidrBlock.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", cidrBlock)
		}

		cidrList = append(cidrList, cidrBlockString)
	}

	return cidrList, nil
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
		return diag.Errorf("error reading cluster list: %s", err)
	}

lookup:
	for i := range clusters {
		c, err := c.GetCluster(ctx, clusters[i].ID)
		if err != nil {
			return diag.Errorf("error reading cluster ID=%d: %s", clusters[i].ID, err)
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
		// cluster was deleted manually
		return nil
	}

	if vpcPeering == nil {
		// vpc peering was deleted manually
		return nil
	}

	if p = c.Meta.ProviderByID(cluster.CloudProviderID); p == nil {
		return diag.Errorf("unable to find cloud provider with id=%d", cluster.CloudProviderID)
	}

	r := p.RegionByID(vpcPeering.RegionID)

	_ = d.Set("datacenter", cluster.Datacenter.Name)
	_ = d.Set("peer_vpc_id", vpcPeering.VPCID)
	_ = d.Set("peer_region", r.ExternalID)
	_ = d.Set("peer_account_id", vpcPeering.OwnerID)
	_ = d.Set("vpc_peering_id", vpcPeering.ID)
	_ = d.Set("connection_id", vpcPeering.ExternalID)
	_ = d.Set("cluster_id", cluster.ID)
	_ = d.Set("network_link", vpcPeering.NetworkLink())
	_ = d.Set("allow_cql", vpcPeering.AllowCQL)

	if c.Meta.GCPBlocks[r.ExternalID] != vpcPeering.CIDRList[0] {
		_ = d.Set("peer_cidr_block", vpcPeering.CIDRList[0])
		_ = d.Set("peer_cidr_blocks", vpcPeering.CIDRList)
	}

	return nil
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
