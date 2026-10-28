package vectorsearch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func ResourceVectorSearch() *schema.Resource {
	return &schema.Resource{
		Description: "Vector search nodes in a cluster datacenter.",

		CreateContext: resourceVectorSearchCreate,
		ReadContext:   resourceVectorSearchRead,
		UpdateContext: resourceVectorSearchUpdate,
		DeleteContext: resourceVectorSearchDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceVectorSearchImport,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID.",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeInt,
			},
			"datacenter": {
				Description: "Datacenter name, e.g. AWS_US_EAST_1.",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"node_count": {
				Description:      "Number of vector search nodes, spread evenly across the cluster's racks (1-27).",
				Required:         true,
				Type:             schema.TypeInt,
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntBetween(1, 27)),
			},
			"node_type": {
				Description: "Instance type of the vector search nodes, e.g. r7g.large.",
				Required:    true,
				Type:        schema.TypeString,
			},
			"datacenter_id": {
				Description: "Datacenter ID.",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"status": {
				Description: "Health of vector search: HEALTHY, DEGRADED, UNHEALTHY or UNKNOWN.",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"availability_zones": {
				Description: "Availability zone IDs that hold vector search nodes.",
				Computed:    true,
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func parseID(id string) (clusterID, dcID int64, err error) {
	parts := strings.Split(id, ":")
	if len(parts) == 2 {
		if clusterID, err = strconv.ParseInt(parts[0], 10, 64); err == nil {
			dcID, err = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	if len(parts) != 2 || err != nil {
		return 0, 0, fmt.Errorf("invalid vector search ID %q, want <cluster_id>:<datacenter_id>", id)
	}
	return clusterID, dcID, nil
}

func findDatacenter(ctx context.Context, c *scylla.Client, clusterID int64, match func(model.Datacenter) bool) (*model.Datacenter, error) {
	dcs, err := c.ListDataCenters(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list datacenters of cluster %d: %w", clusterID, err)
	}
	for i := range dcs {
		if match(dcs[i]) {
			return &dcs[i], nil
		}
	}
	return nil, nil
}

// request resolves node_type to its instance type ID in the datacenter's region.
func request(ctx context.Context, c *scylla.Client, dc *model.Datacenter, d *schema.ResourceData) (*model.VectorSearchRequest, error) {
	nodeType := d.Get("node_type").(string)
	instances, err := c.ListVectorSearchInstances(ctx, dc.CloudProviderID, dc.RegionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vector search instance types: %w", err)
	}
	for _, i := range instances {
		if strings.EqualFold(i.ExternalID, nodeType) {
			return &model.VectorSearchRequest{NodeCount: int64(d.Get("node_count").(int)), DefaultInstanceTypeID: i.ID}, nil
		}
	}
	names := make([]string, 0, len(instances))
	for _, i := range instances {
		names = append(names, i.ExternalID)
	}
	return nil, fmt.Errorf("node_type %q is not available for vector search in this region; available: %s", nodeType, strings.Join(names, ", "))
}

func resourceVectorSearchCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID := int64(d.Get("cluster_id").(int))
	dcName := d.Get("datacenter").(string)

	dc, err := findDatacenter(ctx, c, clusterID, func(dc model.Datacenter) bool { return strings.EqualFold(dc.Name, dcName) })
	if err != nil {
		return diag.FromErr(err)
	}
	if dc == nil {
		return diag.Errorf("cluster %d has no datacenter %q", clusterID, dcName)
	}

	req, err := request(ctx, c, dc, d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := c.AwaitNoInProgressRequests(ctx, clusterID); err != nil {
		return diag.Errorf("failed waiting for in-progress requests of cluster %d: %s", clusterID, err)
	}
	r, err := c.CreateVectorSearch(ctx, clusterID, dc.ID, req)
	if err != nil {
		return diag.Errorf("failed to add vector search to cluster %d: %s", clusterID, err)
	}
	d.SetId(fmt.Sprintf("%d:%d", clusterID, dc.ID))
	if err := c.AwaitClusterRequest(ctx, r.ID); err != nil {
		return diag.Errorf("failed to add vector search to cluster %d: %s", clusterID, err)
	}

	return resourceVectorSearchRead(ctx, d, meta)
}

func resourceVectorSearchRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID, dcID, err := parseID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	vs, err := c.GetVectorSearch(ctx, clusterID, dcID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil
		}
		return diag.Errorf("failed to read vector search of cluster %d: %s", clusterID, err)
	}

	nodes, zones := 0, make([]string, 0, len(vs.AvailabilityZones))
	var instanceTypeID int64
	for _, z := range vs.AvailabilityZones {
		if len(z.Nodes) == 0 {
			continue
		}
		zones = append(zones, z.AZID)
		for _, n := range z.Nodes {
			if n.Status == "DELETED" || n.Status == "PENDING_DELETE" {
				continue
			}
			nodes++
			instanceTypeID = n.InstanceTypeID
		}
	}

	// UNKNOWN with no nodes means vector search is not installed.
	if vs.Status == model.VectorSearchStatusUnknown && nodes == 0 {
		d.SetId("")
		return nil
	}

	_ = d.Set("cluster_id", clusterID)
	_ = d.Set("datacenter_id", dcID)
	_ = d.Set("status", vs.Status)
	_ = d.Set("availability_zones", zones)
	_ = d.Set("node_count", nodes)

	if _, ok := d.GetOk("datacenter"); !ok || instanceTypeID != 0 {
		dc, err := findDatacenter(ctx, c, clusterID, func(dc model.Datacenter) bool { return dc.ID == dcID })
		if err != nil {
			return diag.FromErr(err)
		}
		if dc != nil {
			_ = d.Set("datacenter", dc.Name)
			if instanceTypeID != 0 {
				instances, err := c.ListVectorSearchInstances(ctx, dc.CloudProviderID, dc.RegionID)
				if err != nil {
					return diag.Errorf("failed to list vector search instance types: %s", err)
				}
				for _, i := range instances {
					if i.ID == instanceTypeID {
						_ = d.Set("node_type", i.ExternalID)
					}
				}
			}
		}
	}

	return nil
}

func resourceVectorSearchUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID, dcID, err := parseID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	dc, err := findDatacenter(ctx, c, clusterID, func(dc model.Datacenter) bool { return dc.ID == dcID })
	if err != nil {
		return diag.FromErr(err)
	}
	if dc == nil {
		return diag.Errorf("cluster %d has no datacenter %d", clusterID, dcID)
	}
	req, err := request(ctx, c, dc, d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := c.AwaitNoInProgressRequests(ctx, clusterID); err != nil {
		return diag.Errorf("failed waiting for in-progress requests of cluster %d: %s", clusterID, err)
	}
	r, err := c.UpdateVectorSearch(ctx, clusterID, dcID, req)
	if err != nil {
		return diag.Errorf("failed to resize vector search of cluster %d: %s", clusterID, err)
	}
	if err := c.AwaitClusterRequest(ctx, r.ID); err != nil {
		return diag.Errorf("failed to resize vector search of cluster %d: %s", clusterID, err)
	}

	return resourceVectorSearchRead(ctx, d, meta)
}

func resourceVectorSearchDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID, dcID, err := parseID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := c.AwaitNoInProgressRequests(ctx, clusterID); err != nil {
		if scylla.IsClusterDeletedErr(err) {
			return nil
		}
		return diag.Errorf("failed waiting for in-progress requests of cluster %d: %s", clusterID, err)
	}
	r, err := c.DeleteVectorSearch(ctx, clusterID, dcID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			return nil
		}
		return diag.Errorf("failed to remove vector search from cluster %d: %s", clusterID, err)
	}
	if err := c.AwaitClusterRequest(ctx, r.ID); err != nil {
		return diag.Errorf("failed to remove vector search from cluster %d: %s", clusterID, err)
	}
	return nil
}

func resourceVectorSearchImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	clusterID, dcID, err := parseID(d.Id())
	if err != nil {
		return nil, err
	}
	_ = d.Set("cluster_id", clusterID)
	_ = d.Set("datacenter_id", dcID)
	return []*schema.ResourceData{d}, nil
}
