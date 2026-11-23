package vpcpeering

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
			StateContext: resourceVPCPeeringImport,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(vpcPeeringRetryTimeout),
			Update: schema.DefaultTimeout(vpcPeeringRetryTimeout),
			Delete: schema.DefaultTimeout(vpcPeeringDeleteTimeout),
		},

		CustomizeDiff: resourceVPCPeeringCustomizeDiff,

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
				Description: "Cluster ID. Changing it replaces the peering.",
				Required:    true,
				ForceNew:    true,
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
				Description: "Peer VPC CIDR block list, not empty. Required for AWS. For GCP, it defaults to the auto mode subnet range of `peer_region` when the plan knows `peer_region` and `cluster_id` and the region has such a range, and it is required otherwise. Without it, a configuration that fails this check also fails `terraform destroy`, which then needs `-refresh=false`. Changing it replaces the peering.",
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				MinItems:    1,
				Type:        schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"peer_region": {
				Description:           "Peer VPC region, in any letter case. For AWS, any AWS region, including regions where ScyllaDB Cloud does not deploy clusters; an opt-in region must be enabled in the peer AWS account. For GCP, it is used only to pick the default `peer_cidr_blocks`, but a change still replaces the peering.",
				Required:              true,
				ForceNew:              true,
				Type:                  schema.TypeString,
				DiffSuppressFunc:      suppressRegionCaseDiff,
				DiffSuppressOnRefresh: true,
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
		pr                       = strings.ToLower(d.Get("peer_region").(string))
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

	if dc = findDatacenter(dcs, dcName); dc == nil {
		return diag.Errorf("unable to find %q datacenter", dcName)
	}

	r.DatacenterID = dc.ID

	if p = c.Meta.ProviderByID(dc.CloudProviderID); p == nil {
		return diag.Errorf("unable to find cloud provider with id=%d", dc.CloudProviderID)
	}

	// GCP networks are global: the API ignores the region of a GCP peering,
	// and the provider uses it only to pick the default CIDR block.
	isGCP := strings.EqualFold(p.CloudProvider.Name, "GCP")
	if !isGCP {
		r.Region, r.RegionID = awsPeerRegion(p, pr)
	}

	if !cidrBlocksOK {
		if !isGCP {
			return diag.Errorf(`"peer_cidr_blocks" is required for %q cloud`, p.CloudProvider.Name)
		}

		cidr, ok := c.Meta.GCPBlocks[pr]
		if !ok {
			return diag.FromErr(noDefaultCIDRError(pr))
		}

		cidrBlocks = []any{cidr}
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
	_ = d.Set("peer_cidr_blocks", cidrList)

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

	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return diag.Errorf("error reading cluster list: %s", err)
	}

	// All the clusters of a shared network list its peerings, so a refresh
	// looks in the cluster of the state first.
	var stateCluster, others []model.Cluster
	for _, cl := range clusters {
		if cl.ID == stateClusterID {
			stateCluster = append(stateCluster, cl)
		} else {
			others = append(others, cl)
		}
	}

	cluster, vpcPeering, err := findPeering(ctx, c, stateCluster, connID)
	if err == nil && cluster == nil {
		cluster, vpcPeering, err = findPeering(ctx, c, others, connID)
	}
	if err != nil {
		return diag.FromErr(err)
	}

	if cluster == nil {
		// The peering or its cluster was deleted.
		d.SetId("")
		return nil
	}

	// Taking the peering from another cluster would change cluster_id, which
	// replaces the peering, and removing it from the state would leave it in
	// the cloud.
	if stateClusterID != 0 && cluster.ID != stateClusterID {
		return diag.Errorf("VPC peering %q is not listed by cluster %d of the state but by cluster %d: "+
			"to manage it through that cluster, remove it from the state and import it again", connID, stateClusterID, cluster.ID)
	}

	if p = c.Meta.ProviderByID(cluster.CloudProviderID); p == nil {
		return diag.Errorf("unable to find cloud provider with id=%d", cluster.CloudProviderID)
	}

	// Keep the configured region when the API returns none, as for a GCP peering.
	if region := peerRegionName(p, vpcPeering); region != "" {
		_ = d.Set("peer_region", region)
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
func findPeering(ctx context.Context, c *scylla.Client, clusters []model.Cluster, connID string) (*model.Cluster, *model.VPCPeering, error) {
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

// resourceVPCPeeringCustomizeDiff plans the default CIDR block of peer_region
// when the configuration does not set peer_cidr_blocks. The plan then shows
// the CIDR blocks of a replacement, and removing custom ones replaces the
// peering. A region without a default, as any AWS region, a region known only
// at apply time, a cluster that is not on GCP, or a datacenter that the
// cluster does not have fails the plan before anything is deleted.
func resourceVPCPeeringCustomizeDiff(ctx context.Context, d *schema.ResourceDiff, meta any) error {
	// The state keeps the CIDR blocks of a removed list, so only the raw
	// configuration tells whether it sets them. An unknown list is set.
	config := d.GetRawConfig()
	if config.IsNull() || !config.GetAttr("peer_cidr_blocks").IsNull() {
		return nil
	}

	// An unknown region replaces the peering, and a create without a
	// default would fail only after the delete.
	if !d.NewValueKnown("peer_region") {
		return errors.New(`"peer_cidr_blocks" is required when "peer_region" is known only at apply time`)
	}

	// The default CIDR blocks come with the deployment metadata.
	c := meta.(*scylla.Client)
	if c.Meta == nil {
		return errors.New(`"peer_cidr_blocks" is required when the provider sets "metadata" to false`)
	}

	region := strings.ToLower(d.Get("peer_region").(string))

	cidr, ok := c.Meta.GCPBlocks[region]
	if !ok {
		return noDefaultCIDRError(region)
	}

	// Terraform plans the new peering of a replacement again, without the
	// prior state, so only a new peering is checked against the API. A plan
	// without changes then does not depend on the API.
	if d.Id() == "" {
		if err := requireGCPDatacenter(ctx, d, c); err != nil {
			return err
		}
	}

	return d.SetNew("peer_cidr_blocks", []string{cidr})
}

// requireGCPDatacenter fails unless the cluster of a new peering is on GCP,
// as a peer_region is any string for AWS, and has the datacenter of the
// peering. A datacenter known only at apply time is left to the create. An
// unknown cluster_id fails: it can still be the cluster of the existing
// peering, as when a data source reads it only at apply time, and the plan
// then replaces that peering.
func requireGCPDatacenter(ctx context.Context, d *schema.ResourceDiff, c *scylla.Client) error {
	if !d.NewValueKnown("cluster_id") {
		return errors.New(`"peer_cidr_blocks" is required when "cluster_id" is known only at apply time`)
	}

	clusterID := d.Get("cluster_id").(int)

	dcs, err := c.ListDataCenters(ctx, int64(clusterID))
	if err != nil {
		return fmt.Errorf("error reading the datacenters of cluster %d: %w", clusterID, err)
	}
	if len(dcs) == 0 {
		return fmt.Errorf("cluster %d has no ACTIVE datacenter", clusterID)
	}

	// All datacenters of a cluster are on the same cloud.
	dc := &dcs[0]
	if d.NewValueKnown("datacenter") {
		name := d.Get("datacenter").(string)
		if dc = findDatacenter(dcs, name); dc == nil {
			return fmt.Errorf("unable to find %q datacenter", name)
		}
	}

	p := c.Meta.ProviderByID(dc.CloudProviderID)
	if p == nil {
		return fmt.Errorf("unable to find cloud provider with id=%d", dc.CloudProviderID)
	}
	if !strings.EqualFold(p.CloudProvider.Name, "GCP") {
		return fmt.Errorf(`"peer_cidr_blocks" is required: cluster %d is on %s`, clusterID, p.CloudProvider.Name)
	}

	return nil
}

// findDatacenter returns the datacenter named name in any letter case, or nil.
func findDatacenter(dcs []model.Datacenter, name string) *model.Datacenter {
	i := slices.IndexFunc(dcs, func(dc model.Datacenter) bool {
		return strings.EqualFold(dc.Name, name)
	})
	if i < 0 {
		return nil
	}

	return &dcs[i]
}

func noDefaultCIDRError(region string) error {
	return fmt.Errorf(`"peer_cidr_blocks" is required: %q has no default CIDR block`, region)
}

// resourceVPCPeeringImport imports a peering by its connection ID. The API
// returns no region for a GCP peering, so a GCP import ID also takes the
// peer region: "<connection_id>,<peer_region>".
func resourceVPCPeeringImport(ctx context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
	importID := d.Id()

	// A space kept in peer_region would differ from the configured region
	// and replace the peering on the next plan.
	connID, region, hasRegion := strings.Cut(importID, ",")
	connID, region = strings.TrimSpace(connID), strings.TrimSpace(region)
	if connID == "" || (hasRegion && (region == "" || strings.Contains(region, ","))) {
		return nil, fmt.Errorf(`invalid import ID %q, use "<connection_id>" or "<connection_id>,<peer_region>"`, importID)
	}

	d.SetId(connID)
	if hasRegion {
		_ = d.Set("peer_region", region)
	}

	if diags := resourceVPCPeeringRead(ctx, d, meta); diags.HasError() {
		return nil, fmt.Errorf("%s", diags[0].Summary)
	}

	if d.Id() == "" {
		return nil, fmt.Errorf("VPC peering %q not found", connID)
	}

	if d.Get("peer_region").(string) == "" {
		return nil, fmt.Errorf(`the API returns no region for VPC peering %q, import it as "%s,<peer_region>"`, connID, connID)
	}

	return []*schema.ResourceData{d}, nil
}

func resourceVPCPeeringUpgradeV0(ctx context.Context, rawState map[string]interface{}, meta interface{}) (map[string]interface{}, error) {
	if cidr, ok := rawState["peer_cidr_block"].(string); ok {
		rawState["peer_cidr_blocks"] = split(cidr, ",")
		delete(rawState, "peer_cidr_block")
	}

	return rawState, nil
}

// awsPeerRegion returns the region name and ID to send for an AWS peering.
// The ID is set only for a region where ScyllaDB Cloud deploys clusters, so
// an API that does not read the name yet still gets the region. The API
// validates any other name.
func awsPeerRegion(p *scylla.CloudProvider, name string) (string, int64) {
	r := p.RegionByName(name)
	if r == nil {
		return name, 0
	}

	return r.ExternalID, r.ID
}

// peerRegionName returns the peer region of a peering read from the API: its
// name, or the region of its ID from an API that does not return the name.
// It is empty when the API returns neither.
func peerRegionName(p *scylla.CloudProvider, vp *model.VPCPeering) string {
	if vp.Region != "" {
		return vp.Region
	}
	if vp.RegionID == 0 {
		return ""
	}

	if r := p.RegionByID(vp.RegionID); r != nil {
		return r.ExternalID
	}
	return ""
}

// suppressRegionCaseDiff ignores a peer_region change in letter case only.
// The API keeps region names in lowercase.
func suppressRegionCaseDiff(_, oldValue, newValue string, _ *schema.ResourceData) bool {
	return strings.EqualFold(oldValue, newValue)
}

func split(s, sep string) (x []any) {
	for _, v := range strings.Split(s, sep) {
		if v := strings.TrimSpace(v); v != "" {
			x = append(x, v)
		}
	}
	return x
}
