package datasource

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func cloudArg() *schema.Schema {
	return &schema.Schema{Type: schema.TypeString, Optional: true, Default: "AWS", Description: "Cloud provider, AWS or GCP."}
}

func regionArg() *schema.Schema {
	return &schema.Schema{Type: schema.TypeString, Required: true, Description: "Cloud region, e.g. us-east-1."}
}

func findProvider(ctx context.Context, c *scylla.Client, name string) (*model.CloudProvider, error) {
	providers, err := c.ListCloudProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list cloud providers: %w", err)
	}
	for i := range providers {
		if strings.EqualFold(providers[i].Name, name) {
			return &providers[i], nil
		}
	}
	return nil, fmt.Errorf("unknown cloud %q", name)
}

func findRegion(ctx context.Context, c *scylla.Client, d *schema.ResourceData) (*model.CloudProvider, *model.CloudProviderRegion, error) {
	p, err := findProvider(ctx, c, d.Get("cloud").(string))
	if err != nil {
		return nil, nil, err
	}
	regions, err := c.ListCloudProviderRegions(ctx, p.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list %s regions: %w", p.Name, err)
	}
	name := d.Get("region").(string)
	for i := range regions.Regions {
		if strings.EqualFold(regions.Regions[i].ExternalID, name) {
			return p, &regions.Regions[i], nil
		}
	}
	return nil, nil, fmt.Errorf("unknown %s region %q", p.Name, name)
}

func DataSourceCloudProviders() *schema.Resource {
	return &schema.Resource{
		Description: "The cloud providers ScyllaDB Cloud deploys to.",
		ReadContext: dataSourceCloudProvidersRead,
		Schema: map[string]*schema.Schema{
			"cloud_providers": computedList("Cloud providers.", map[string]*schema.Schema{
				"cloud_provider_id": computed(schema.TypeInt, "Cloud provider ID."),
				"name":              computed(schema.TypeString, "Name, e.g. AWS."),
			}),
		},
	}
}

func dataSourceCloudProvidersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	providers, err := meta.(*scylla.Client).ListCloudProviders(ctx)
	if err != nil {
		return diag.Errorf("failed to list cloud providers: %s", err)
	}
	out := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		out = append(out, map[string]interface{}{"cloud_provider_id": p.ID, "name": p.Name})
	}
	d.SetId("cloud-providers")
	_ = d.Set("cloud_providers", out)
	return nil
}

func DataSourceRegions() *schema.Resource {
	return &schema.Resource{
		Description: "The regions of a cloud provider.",
		ReadContext: dataSourceRegionsRead,
		Schema: map[string]*schema.Schema{
			"cloud": cloudArg(),
			"regions": computedList("Regions.", map[string]*schema.Schema{
				"region_id":  computed(schema.TypeInt, "Region ID."),
				"name":       computed(schema.TypeString, "Region, e.g. us-east-1."),
				"full_name":  computed(schema.TypeString, "Descriptive name."),
				"continent":  computed(schema.TypeString, "Continent."),
				"datacenter": computed(schema.TypeString, "Datacenter name clusters in this region get."),
			}),
		},
	}
}

func dataSourceRegionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	p, err := findProvider(ctx, c, d.Get("cloud").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	regions, err := c.ListCloudProviderRegions(ctx, p.ID)
	if err != nil {
		return diag.Errorf("failed to list %s regions: %s", p.Name, err)
	}
	out := make([]map[string]interface{}, 0, len(regions.Regions))
	for _, r := range regions.Regions {
		out = append(out, map[string]interface{}{
			"region_id": r.ID, "name": r.ExternalID, "full_name": r.FullName, "continent": r.Continent, "datacenter": r.DatacenterName,
		})
	}
	d.SetId(p.Name)
	_ = d.Set("regions", out)
	return nil
}

func DataSourceInstanceTypes() *schema.Resource {
	return &schema.Resource{
		Description: "The instance types available in a region, for cluster nodes or for vector search nodes.",
		ReadContext: dataSourceInstanceTypesRead,
		Schema: map[string]*schema.Schema{
			"cloud":  cloudArg(),
			"region": regionArg(),
			"target": {
				Type: schema.TypeString, Optional: true, Default: "CLUSTER",
				Description:      "CLUSTER for cluster nodes or VECTOR_SEARCH for vector search nodes.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"CLUSTER", "VECTOR_SEARCH"}, false)),
			},
			"instance_types": computedList("Instance types.", map[string]*schema.Schema{
				"instance_type_id": computed(schema.TypeInt, "Instance type ID."),
				"name":             computed(schema.TypeString, "Instance type, e.g. i8g.large."),
				"family":           computed(schema.TypeString, "Instance family."),
				"cpu_count":        computed(schema.TypeInt, "vCPUs."),
				"memory":           computed(schema.TypeInt, "Memory in MB."),
				"total_storage":    computed(schema.TypeInt, "Local storage in GB."),
			}),
		},
	}
}

func dataSourceInstanceTypesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	p, r, err := findRegion(ctx, c, d)
	if err != nil {
		return diag.FromErr(err)
	}
	list := c.ListCloudProviderInstancesPerRegion
	if d.Get("target").(string) == "VECTOR_SEARCH" {
		list = c.ListVectorSearchInstances
	}
	instances, err := list(ctx, p.ID, r.ID)
	if err != nil {
		return diag.Errorf("failed to list instance types: %s", err)
	}
	out := make([]map[string]interface{}, 0, len(instances))
	for _, i := range instances {
		out = append(out, map[string]interface{}{
			"instance_type_id": i.ID, "name": i.ExternalID, "family": i.Family,
			"cpu_count": i.CPUCount, "memory": i.Memory, "total_storage": i.TotalStorage,
		})
	}
	d.SetId(fmt.Sprintf("%s/%s/%s", p.Name, r.ExternalID, d.Get("target")))
	_ = d.Set("instance_types", out)
	return nil
}

func DataSourceScyllaVersions() *schema.Resource {
	return &schema.Resource{
		Description: "The Scylla versions new clusters can run.",
		ReadContext: dataSourceScyllaVersionsRead,
		Schema: map[string]*schema.Schema{
			"default_version": computed(schema.TypeString, "Version new clusters get by default."),
			"versions": computedList("Versions.", map[string]*schema.Schema{
				"version_id":  computed(schema.TypeInt, "Version ID."),
				"version":     computed(schema.TypeString, "Version, e.g. 2025.1.2."),
				"description": computed(schema.TypeString, "Description."),
			}),
		},
	}
}

func dataSourceScyllaVersionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	versions, err := meta.(*scylla.Client).ListScyllaVersions(ctx)
	if err != nil {
		return diag.Errorf("failed to list Scylla versions: %s", err)
	}
	out := make([]map[string]interface{}, 0, len(versions.ScyllaVersions))
	for _, v := range versions.ScyllaVersions {
		out = append(out, map[string]interface{}{"version_id": v.ID, "version": v.Version, "description": v.Description})
		if v.ID == versions.DefaultScyllaVersionID {
			_ = d.Set("default_version", v.Version)
		}
	}
	d.SetId("scylla-versions")
	_ = d.Set("versions", out)
	return nil
}

func DataSourceAvailabilityZones() *schema.Resource {
	return &schema.Resource{
		Description: "The availability zone IDs clusters can use in a region.",
		ReadContext: dataSourceAvailabilityZonesRead,
		Schema: map[string]*schema.Schema{
			"cloud":  cloudArg(),
			"region": regionArg(),
			"cloud_account_id": {
				Type: schema.TypeInt, Optional: true,
				Description: "Cloud account to list zones for; defaults to ScyllaDB's account for the cloud.",
			},
			"zone_ids": stringList("Availability zone IDs, e.g. use1-az1."),
		},
	}
}

func dataSourceAvailabilityZonesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	p, r, err := findRegion(ctx, c, d)
	if err != nil {
		return diag.FromErr(err)
	}
	accountID := int64(d.Get("cloud_account_id").(int))
	if accountID == 0 {
		accounts, err := c.ListCloudAccounts(ctx)
		if err != nil {
			return diag.Errorf("failed to list cloud accounts: %s", err)
		}
		a := model.FindScyllaCloudAccount(accounts, p.ID)
		if a == nil {
			return diag.Errorf("no active ScyllaDB cloud account for %s", p.Name)
		}
		accountID = a.ID
	}
	zones, err := c.ListAvailabilityZoneIDs(ctx, accountID, r.ID)
	if err != nil {
		return diag.Errorf("failed to list availability zones: %s", err)
	}
	d.SetId(fmt.Sprintf("%d/%s", accountID, r.ExternalID))
	_ = d.Set("cloud_account_id", accountID)
	_ = d.Set("zone_ids", zones)
	return nil
}

func DataSourceClusterRequests() *schema.Resource {
	return &schema.Resource{
		Description: "The requests (create, resize, delete and others) of a cluster.",
		ReadContext: dataSourceClusterRequestsRead,
		Schema: map[string]*schema.Schema{
			"cluster_id": clusterIDArg(),
			"status": {
				Type: schema.TypeString, Optional: true,
				Description: "Only requests in this status, e.g. IN_PROGRESS.",
			},
			"requests": computedList("Requests.", map[string]*schema.Schema{
				"request_id":           computed(schema.TypeInt, "Request ID."),
				"type":                 computed(schema.TypeString, "Request type, e.g. CREATE_CLUSTER."),
				"status":               computed(schema.TypeString, "Request status."),
				"progress_percent":     computed(schema.TypeInt, "Progress, 0-100."),
				"progress_description": computed(schema.TypeString, "Current step."),
			}),
		},
	}
}

func dataSourceClusterRequestsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := int64(d.Get("cluster_id").(int))
	status := d.Get("status").(string)
	reqs, err := meta.(*scylla.Client).ListClusterRequest(ctx, id, scylla.ListClusterRequestParams{Status: status})
	if err != nil {
		return diag.Errorf("failed to list requests of cluster %d: %s", id, err)
	}
	out := make([]map[string]interface{}, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, map[string]interface{}{
			"request_id": r.ID, "type": r.RequestType, "status": r.Status,
			"progress_percent": r.ProgressPercent, "progress_description": r.ProgressDescription,
		})
	}
	d.SetId(strconv.FormatInt(id, 10) + "/" + status)
	_ = d.Set("requests", out)
	return nil
}
