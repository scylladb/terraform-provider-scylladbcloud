package vectorsearch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const vsOpTimeout = 90 * time.Minute

// ResourceVectorSearch returns the schema.Resource for scylladbcloud_vector_search.
func ResourceVectorSearch() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVectorSearchCreate,
		ReadContext:   resourceVectorSearchRead,
		UpdateContext: resourceVectorSearchUpdate,
		DeleteContext: resourceVectorSearchDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(vsOpTimeout),
			Update: schema.DefaultTimeout(vsOpTimeout),
			Delete: schema.DefaultTimeout(vsOpTimeout),
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the cluster to install vector search on.",
			},
			"datacenter_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "ID of the datacenter. If not specified, derived from the cluster's first datacenter.",
			},
			"node_count": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Number of vector search nodes (1-27).",
			},
			"node_type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Node type name for vector search nodes (e.g., \"t4g.large\").",
			},
		},
	}
}

func resourceVectorSearchCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID := int64(d.Get("cluster_id").(int))
	nodeCount := d.Get("node_count").(int)
	nodeTypeName := d.Get("node_type").(string)

	// Resolve datacenter ID.
	dcID, diags := resolveDCID(ctx, c, clusterID, d)
	if diags.HasError() {
		return diags
	}

	// Resolve instance type name to ID.
	nodeTypeID, diags := resolveNodeTypeID(ctx, c, clusterID, nodeTypeName)
	if diags.HasError() {
		return diags
	}

	req := &model.VectorSearchRequest{
		NodeCount:             nodeCount,
		DefaultInstanceTypeID: nodeTypeID,
	}

	tflog.Info(ctx, "installing vector search", map[string]interface{}{
		"cluster_id":    clusterID,
		"datacenter_id": dcID,
		"node_count":    nodeCount,
		"node_type":     nodeTypeName,
	})

	cr, err := c.InstallVectorSearch(ctx, clusterID, dcID, req)
	if err != nil {
		return diag.Errorf("failed to install vector search: %s", err)
	}

	if err := cluster.WaitForClusterRequestID(ctx, c, cr.ID); err != nil {
		return diag.Errorf("failed waiting for vector search installation: %s", err)
	}

	d.SetId(formatID(clusterID, dcID))
	_ = d.Set("datacenter_id", int(dcID))

	return resourceVectorSearchRead(ctx, d, meta)
}

func resourceVectorSearchRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, dcID, diags := parseID(d.Id())
	if diags.HasError() {
		return diags
	}

	info, err := c.GetVectorSearchInfo(ctx, clusterID, dcID)
	if err != nil {
		if scylla.IsNotFound(err) || scylla.IsClusterDeletedErr(err) {
			tflog.Warn(ctx, "vector search not found, removing from state")
			d.SetId("")
			return nil
		}
		return diag.Errorf("failed to read vector search info: %s", err)
	}

	// Count active nodes and get instance type ID from the first active node.
	activeNodes := 0
	var instanceTypeID int64
	for _, az := range info.AvailabilityZones {
		for _, node := range az.Nodes {
			if node.Status == "DELETED" || node.Status == "PENDING_DELETE" {
				continue
			}
			activeNodes++
			if instanceTypeID == 0 {
				instanceTypeID = node.InstanceTypeID
			}
		}
	}

	// If no active nodes found, treat as deleted.
	if activeNodes == 0 {
		tflog.Warn(ctx, "no active vector search nodes found, removing from state")
		d.SetId("")
		return nil
	}

	// Resolve instance type ID back to name.
	nodeTypeName, diags := resolveNodeTypeName(ctx, c, clusterID, instanceTypeID)
	if diags.HasError() {
		return diags
	}

	_ = d.Set("cluster_id", int(clusterID))
	_ = d.Set("datacenter_id", int(dcID))
	_ = d.Set("node_count", activeNodes)
	_ = d.Set("node_type", nodeTypeName)

	return nil
}

func resourceVectorSearchUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, dcID, diags := parseID(d.Id())
	if diags.HasError() {
		return diags
	}

	nodeCount := d.Get("node_count").(int)
	instanceTypeName := d.Get("node_type").(string)

	instanceTypeID, diags := resolveNodeTypeID(ctx, c, clusterID, instanceTypeName)
	if diags.HasError() {
		return diags
	}

	req := &model.VectorSearchRequest{
		NodeCount:             nodeCount,
		DefaultInstanceTypeID: instanceTypeID,
	}

	tflog.Info(ctx, "resizing vector search", map[string]interface{}{
		"cluster_id":    clusterID,
		"datacenter_id": dcID,
		"node_count":    nodeCount,
		"node_type":     instanceTypeName,
	})

	cr, err := c.ResizeVectorSearch(ctx, clusterID, dcID, req)
	if err != nil {
		return diag.Errorf("failed to resize vector search: %s", err)
	}

	if err := cluster.WaitForClusterRequestID(ctx, c, cr.ID); err != nil {
		return diag.Errorf("failed waiting for vector search resize: %s", err)
	}

	return resourceVectorSearchRead(ctx, d, meta)
}

func resourceVectorSearchDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, dcID, diags := parseID(d.Id())
	if diags.HasError() {
		return diags
	}

	tflog.Info(ctx, "deleting vector search", map[string]interface{}{
		"cluster_id":    clusterID,
		"datacenter_id": dcID,
	})

	cr, err := c.DeleteVectorSearch(ctx, clusterID, dcID)
	if err != nil {
		if scylla.IsNotFound(err) || scylla.IsDeletedErr(err) || scylla.IsClusterDeletedErr(err) {
			return nil // Already gone, nothing to do.
		}
		return diag.Errorf("failed to delete vector search: %s", err)
	}

	if err := cluster.WaitForClusterRequestID(ctx, c, cr.ID); err != nil {
		return diag.Errorf("failed waiting for vector search deletion: %s", err)
	}

	return nil
}

// resolveDCID returns the datacenter ID from the schema or derives it from the cluster.
func resolveDCID(ctx context.Context, c *scylla.Client, clusterID int64, d *schema.ResourceData) (int64, diag.Diagnostics) {
	if v, ok := d.GetOk("datacenter_id"); ok {
		return int64(v.(int)), nil
	}

	dcs, err := c.ListDataCenters(ctx, clusterID)
	if err != nil {
		return 0, diag.Errorf("failed to list datacenters for cluster %d: %s", clusterID, err)
	}
	if len(dcs) == 0 {
		return 0, diag.Errorf("cluster %d has no datacenters", clusterID)
	}

	return dcs[0].ID, nil
}

// resolveNodeTypeID resolves a human-readable instance type name to its numeric ID
// using the vector-search-specific instance type list.
func resolveNodeTypeID(ctx context.Context, c *scylla.Client, clusterID int64, name string) (int64, diag.Diagnostics) {
	providerID, regionID, diags := getClusterProviderAndRegion(ctx, c, clusterID)
	if diags.HasError() {
		return 0, diags
	}

	instances, err := c.ListVectorSearchInstances(ctx, providerID, regionID)
	if err != nil {
		return 0, diag.Errorf("failed to list vector search instance types: %s", err)
	}

	for _, inst := range instances {
		if strings.EqualFold(inst.ExternalID, name) {
			return inst.ID, nil
		}
	}

	available := make([]string, 0, len(instances))
	for _, inst := range instances {
		available = append(available, inst.ExternalID)
	}

	return 0, diag.Errorf("instance type %q not found for vector search; available: %v", name, available)
}

// resolveNodeTypeName resolves a numeric instance type ID back to its human-readable name.
func resolveNodeTypeName(ctx context.Context, c *scylla.Client, clusterID, instanceTypeID int64) (string, diag.Diagnostics) {
	providerID, regionID, diags := getClusterProviderAndRegion(ctx, c, clusterID)
	if diags.HasError() {
		return "", diags
	}

	instances, err := c.ListVectorSearchInstances(ctx, providerID, regionID)
	if err != nil {
		return "", diag.Errorf("failed to list vector search instance types: %s", err)
	}

	for _, inst := range instances {
		if inst.ID == instanceTypeID {
			return inst.ExternalID, nil
		}
	}

	return "", diag.Errorf("instance type with ID %d not found in vector search instance types", instanceTypeID)
}

// getClusterProviderAndRegion fetches the cloud provider and region IDs for a cluster.
func getClusterProviderAndRegion(ctx context.Context, c *scylla.Client, clusterID int64) (providerID, regionID int64, diags diag.Diagnostics) {
	dcs, err := c.ListDataCenters(ctx, clusterID)
	if err != nil {
		return 0, 0, diag.Errorf("failed to list datacenters for cluster %d: %s", clusterID, err)
	}
	if len(dcs) == 0 {
		return 0, 0, diag.Errorf("cluster %d has no datacenters", clusterID)
	}

	dc := dcs[0]
	return dc.CloudProviderID, dc.RegionID, nil
}

// formatID creates a composite resource ID from cluster and datacenter IDs.
func formatID(clusterID, dcID int64) string {
	return fmt.Sprintf("%d/%d", clusterID, dcID)
}

// parseID extracts cluster and datacenter IDs from a composite resource ID.
func parseID(id string) (clusterID, dcID int64, diags diag.Diagnostics) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return 0, 0, diag.Errorf("invalid vector search resource ID %q, expected format: <cluster_id>/<datacenter_id>", id)
	}

	clusterID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, diag.Errorf("invalid cluster_id in resource ID %q: %s", id, err)
	}

	dcID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, diag.Errorf("invalid datacenter_id in resource ID %q: %s", id, err)
	}

	return clusterID, dcID, nil
}
