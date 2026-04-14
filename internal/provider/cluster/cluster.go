package cluster

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	clusterRetryTimeout  = 40 * time.Minute
	clusterDeleteTimeout = 90 * time.Minute
	clusterPollInterval  = 10 * time.Second
)

func validateMinNodesDiag(v interface{}, _ cty.Path) diag.Diagnostics {
	value := v.(int)
	if value < 3 {
		return diag.Errorf("min_nodes must be at least 3, got %d", value)
	}
	if value%3 != 0 {
		return diag.Errorf("min_nodes must be divisible by 3, got %d", value)
	}
	return nil
}

func validateScalingTargetUtilizationDiag(v interface{}, _ cty.Path) diag.Diagnostics {
	value := v.(float64)
	if value <= 0.0 || value > 1.0 {
		return diag.Errorf("target_utilization must be greater than 0.0 and less than or equal to 1.0, got %v", value)
	}
	return nil
}

func nestedBlock(raw interface{}) (map[string]interface{}, bool) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 || items[0] == nil {
		return nil, false
	}

	block, ok := items[0].(map[string]interface{})
	return block, ok
}

func nonEmptyList(raw interface{}) bool {
	items, ok := raw.([]interface{})
	return ok && len(items) > 0
}

func int64List(raw interface{}) []int64 {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}

	out := make([]int64, 0, len(items))
	for _, item := range items {
		out = append(out, int64(item.(int)))
	}

	return out
}

func stringList(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.(string))
	}

	return out
}

func expandScaling(raw interface{}, region string, instances []model.CloudProviderInstance, cloudProvider *scylla.CloudProvider) (*model.DatacenterScaling, error) {
	block, ok := nestedBlock(raw)
	if !ok {
		return nil, nil
	}

	scaling := &model.DatacenterScaling{
		InstanceFamilies: stringList(block["instance_families"]),
	}

	if instanceTypes := stringList(block["instance_types"]); len(instanceTypes) > 0 {
		scaling.InstanceTypeIDs = make([]int64, 0, len(instanceTypes))
		for _, instanceType := range instanceTypes {
			instance := cloudProvider.InstanceByNameFromInstances(instanceType, instances)
			if instance == nil {
				return nil, fmt.Errorf("unsupported scaling instance_type %q in region %s", instanceType, region)
			}
			scaling.InstanceTypeIDs = append(scaling.InstanceTypeIDs, instance.ID)
		}
	}

	if storagePolicy, ok := nestedBlock(block["storage_policy"]); ok {
		if scaling.Policies == nil {
			scaling.Policies = &model.DatacenterScalingPolicies{}
		}
		scaling.Policies.Storage = &model.DatacenterScalingStoragePolicy{
			Min:               int64(storagePolicy["min"].(int)),
			TargetUtilization: storagePolicy["target_utilization"].(float64),
		}
	}

	if vcpuPolicy, ok := nestedBlock(block["vcpu_policy"]); ok {
		if scaling.Policies == nil {
			scaling.Policies = &model.DatacenterScalingPolicies{}
		}
		scaling.Policies.VCPU = &model.DatacenterScalingVCPUPolicy{
			Min: int64(vcpuPolicy["min"].(int)),
		}
	}

	if !scaling.Enabled() {
		return nil, nil
	}

	return scaling, nil
}

func clusterUsesScaling(cluster *model.Cluster) bool {
	if cluster == nil {
		return false
	}

	if cluster.Datacenter != nil && cluster.Datacenter.Scaling != nil && cluster.Datacenter.Scaling.Enabled() {
		return true
	}

	if len(cluster.Datacenters) == 1 && cluster.Datacenters[0].Scaling != nil && cluster.Datacenters[0].Scaling.Enabled() {
		return true
	}

	return cluster.ScalingMode != nil && strings.EqualFold(cluster.ScalingMode.Mode, "xcloud")
}

func validateClusterSizingMode(hasScaling, hasMinNodes, hasNodeType bool, scaling map[string]interface{}) error {
	if hasScaling {
		if hasMinNodes {
			return fmt.Errorf(`"scaling" cannot be used together with "min_nodes"`)
		}
		if hasNodeType {
			return fmt.Errorf(`"scaling" cannot be used together with "node_type"`)
		}

		hasInstanceFamilies := nonEmptyList(scaling["instance_families"])
		hasInstanceTypes := nonEmptyList(scaling["instance_types"])

		if hasInstanceFamilies == hasInstanceTypes {
			return fmt.Errorf(`exactly one of "instance_families" or "instance_types" must be configured in the "scaling" block`)
		}

		return nil
	}

	if !hasMinNodes && !hasNodeType {
		return fmt.Errorf(`either configure the "scaling" block or set both "min_nodes" and "node_type"`)
	}
	if !hasMinNodes {
		return fmt.Errorf(`"min_nodes" is required when the "scaling" block is not configured`)
	}
	if !hasNodeType {
		return fmt.Errorf(`"node_type" is required when the "scaling" block is not configured`)
	}

	return nil
}

func resourceClusterCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	scaling, hasScaling := nestedBlock(d.Get("scaling"))
	_, hasMinNodes := d.GetOk("min_nodes")
	_, hasNodeType := d.GetOk("node_type")

	return validateClusterSizingMode(hasScaling, hasMinNodes, hasNodeType, scaling)
}

func ResourceCluster() *schema.Resource {
	tflog.Info(context.Background(), "Read ResourceCluster")
	return &schema.Resource{
		CreateContext: resourceClusterCreate,
		ReadContext:   resourceClusterRead,
		UpdateContext: resourceClusterUpdate,
		DeleteContext: resourceClusterDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(clusterRetryTimeout),
			Update: schema.DefaultTimeout(clusterRetryTimeout),
			Delete: schema.DefaultTimeout(clusterDeleteTimeout),
		},

		CustomizeDiff: resourceClusterCustomizeDiff,

		SchemaVersion: 3,

		StateUpgraders: []schema.StateUpgrader{
			{
				Version: 0,
				Type:    resourceClusterV0().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceClusterUpgradeV0,
			},
			{
				Version: 1,
				Type:    resourceClusterV1().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceClusterUpgradeV1,
			},
			{
				Version: 2,
				Type:    resourceClusterV2().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceClusterUpgradeV2,
			},
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster id",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"cloud": {
				Description: "Cloud provider (AWS, GCP)",
				Optional:    true,
				ForceNew:    true,
				Default:     "AWS",
				Type:        schema.TypeString,
			},
			"name": {
				Description: "Cluster name",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"region": {
				Description: "Region to use",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"node_count": {
				Description: "Current node count (computed)",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"min_nodes": {
				Description:      "Minimum number of nodes for regular clusters",
				Optional:         true,
				Type:             schema.TypeInt,
				ConflictsWith:    []string{"scaling"},
				ValidateDiagFunc: validateMinNodesDiag,
			},
			"byoa_id": {
				Description: "BYOA credential ID (only for AWS)",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeInt,
			},
			"user_api_interface": {
				Description: "Type of API interface, either CQL or ALTERNATOR",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
				Default:     "CQL",
			},
			"alternator_write_isolation": {
				Description: "Default write isolation policy",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
				Default:     "only_rmw_uses_lwt",
			},
			"node_type": {
				Description:   "Instance type of a node for regular clusters",
				Optional:      true,
				ForceNew:      true,
				Type:          schema.TypeString,
				ConflictsWith: []string{"scaling"},
			},
			"scaling": {
				Description:   "X Cloud scaling configuration for the single supported datacenter",
				Optional:      true,
				Type:          schema.TypeList,
				MaxItems:      1,
				ConflictsWith: []string{"min_nodes", "node_type"},
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"instance_families": {
						Description: "Allowed instance families for X Cloud scaling",
						Optional:    true,
						Type:        schema.TypeList,
						MinItems:    1,
						Elem:        &schema.Schema{Type: schema.TypeString},
					},
					"instance_types": {
						Description: "Allowed instance types for X Cloud scaling",
						Optional:    true,
						Type:        schema.TypeList,
						MinItems:    1,
						Elem:        &schema.Schema{Type: schema.TypeString},
					},
					"storage_policy": {
						Description: "Storage scaling policy",
						Optional:    true,
						Type:        schema.TypeList,
						MaxItems:    1,
						Elem: &schema.Resource{Schema: map[string]*schema.Schema{
							"min": {
								Description: "Minimum storage in GB",
								Required:    true,
								Type:        schema.TypeInt,
							},
							"target_utilization": {
								Description:      "Target storage utilization ratio",
								Required:         true,
								Type:             schema.TypeFloat,
								ValidateDiagFunc: validateScalingTargetUtilizationDiag,
							},
						}},
					},
					"vcpu_policy": {
						Description: "vCPU scaling policy",
						Optional:    true,
						Type:        schema.TypeList,
						MaxItems:    1,
						Elem: &schema.Resource{Schema: map[string]*schema.Schema{
							"min": {
								Description: "Minimum vCPUs",
								Required:    true,
								Type:        schema.TypeInt,
							},
						}},
					},
				}},
			},
			"node_dns_names": {
				Description: "Cluster nodes DNS names",
				Computed:    true,
				Type:        schema.TypeSet,
				Elem:        schema.TypeString,
				Set:         schema.HashString,
			},
			"node_private_ips": {
				Description: "Cluster nodes private IP addresses",
				Computed:    true,
				Type:        schema.TypeSet,
				Elem:        schema.TypeString,
				Set:         schema.HashString,
			},
			"cidr_block": {
				Description: "IPv4 CIDR of the cluster",
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"scylla_version": {
				Description: "Scylla version",
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"enable_vpc_peering": {
				Description: "Whether to enable VPC peering",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeBool,
				Default:     true,
			},
			"enable_dns": {
				Description: "Whether to enable CNAME for seed nodes",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeBool,
				Default:     true,
			},
			"request_id": {
				Description: "Cluster creation request ID",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"datacenter": {
				Description: "Cluster datacenter name",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"status": {
				Description: "Cluster status",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"node_disk_size": {
				Description: "The disk size in gigabytes of the node",
				ForceNew:    true,
				Optional:    true,
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"availability_zone_ids": {
				Description: "List of Availability Zone IDs for the cluster nodes (e.g., " +
					"'use1-az1', 'use1-az2', 'use1-az4' for AWS or 'us-central1-a', 'us-central1-b', " +
					"'us-central1-c' for GCP). Between 1 and 3 AZ IDs can be specified. It is " +
					"recommended to specify exactly 3 AZ IDs to ensure optimal distribution of " +
					"nodes across availability zones. AZ IDs are consistent identifiers that map " +
					"to the same physical availability zone across all accounts, unlike AZ names " +
					"which may differ between accounts. If not specified, the server will " +
					"automatically select availability zones.",
				Optional: true,
				Computed: true,
				ForceNew: true,
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Info(ctx, "Read resourceClusterCreate")
	var (
		scyllaClient         = meta.(*scylla.Client)
		clusterCreateRequest = &model.ClusterCreateRequest{
			ClusterName:          d.Get("name").(string),
			BroadcastType:        "PRIVATE",
			ReplicationFactor:    3,
			UserAPIInterface:     d.Get("user_api_interface").(string),
			EnableDNSAssociation: d.Get("enable_dns").(bool),
			Placement:            "true",
		}
		cloud                        = d.Get("cloud").(string)
		cidr, cidrOK                 = d.GetOk("cidr_block")
		byoa, byoaOK                 = d.GetOk("byoa_id")
		region                       = d.Get("region").(string)
		nodeType, nodeTypeOK         = d.GetOk("node_type")
		scaling                      *model.DatacenterScaling
		version, versionOK           = d.GetOk("scylla_version")
		enableVpcPeering             = d.Get("enable_vpc_peering").(bool)
		nodeDiskSize, nodeDiskSizeOK = d.GetOk("node_disk_size")
	)

	if !enableVpcPeering {
		clusterCreateRequest.BroadcastType = "PUBLIC"
	}

	if clusterCreateRequest.UserAPIInterface == "ALTERNATOR" {
		clusterCreateRequest.AlternatorWriteIsolation = d.Get("alternator_write_isolation").(string)
	}

	if byoaOK {
		clusterCreateRequest.AccountCredentialID = int64(byoa.(int))
	}

	if scaling != nil {
		clusterCreateRequest.Scaling = scaling
	} else {
		clusterCreateRequest.NumberOfNodes = int64(d.Get("min_nodes").(int))
	}

	if !cidrOK {
		cidr = "172.31.0.0/16"
		_ = d.Set("cidr_block", cidr)
	}

	cloudProvider := scyllaClient.Meta.ProviderByName(cloud)
	if cloudProvider == nil {
		return diag.Errorf(`unrecognized value %q for "cloud" attribute`, cloud)
	}

	clusterCreateRequest.CidrBlock = cidr.(string)

	clusterCreateRequest.CloudProviderID = cloudProvider.CloudProvider.ID

	mr := cloudProvider.RegionByName(region)
	if mr != nil {
		clusterCreateRequest.RegionID = mr.ID
	} else {
		return diag.Errorf(`unrecognized value %q for "region" attribute`, region)
	}

	instances, err := scyllaClient.ListCloudProviderInstancesPerRegion(ctx, cloudProvider.CloudProvider.ID, mr.ID)
	if err != nil {
		return diag.Errorf("failed to list cloud provider instances for region %q: %s", region, err)
	}

	scaling, err = expandScaling(d.Get("scaling"), region, instances, cloudProvider)
	if err != nil {
		return diag.FromErr(err)
	}

	var mi *model.CloudProviderInstance
	if scaling == nil {
		nodeTypeStr := nodeType.(string)
		if nodeDiskSizeOK {
			if mi = cloudProvider.InstanceByNameAndDiskSizeFromInstances(nodeTypeStr, nodeDiskSize.(int), instances); mi == nil {
				return diag.Errorf(
					`unrecognized value combination: %q for "node_type" and %d for "node_disk_size" attributes`,
					nodeTypeStr,
					nodeDiskSize,
				)
			}
		} else {
			if mi = cloudProvider.InstanceByNameFromInstances(nodeTypeStr, instances); mi == nil {
				return diag.Errorf(`unsupported node_type %q in region %s`, nodeTypeStr, mr.ExternalID)
			}
		}

		clusterCreateRequest.InstanceID = mi.ID
	} else if nodeDiskSizeOK || nodeTypeOK {
		return diag.Errorf(`"node_type" and "node_disk_size" are not supported when the "scaling" block is configured`)
	}

	// Handle availability zone IDs
	if azIDs, ok := d.GetOk("availability_zone_ids"); ok {
		// Figure out the cloud account ID; it's either BYOA or Scylla Account.
		// If cloudAccountID is 0, we look up the active cloud account owned by Scylla.
		cloudAccountID := clusterCreateRequest.AccountCredentialID
		if cloudAccountID == 0 {
			cloudAccounts, err := scyllaClient.ListCloudAccounts(ctx)
			if err != nil {
				return diag.Errorf("failed to list cloud accounts: %s", err)
			}

			ca := model.FindScyllaCloudAccount(cloudAccounts, cloudProvider.CloudProvider.ID)
			if ca == nil {
				return diag.Errorf(
					"no active Scylla-owned cloud account found for cloud provider %q (ID %d)",
					cloud, cloudProvider.CloudProvider.ID,
				)
			}
			cloudAccountID = ca.ID
		}

		azIDsSet := azIDs.(*schema.Set)

		var azIDList []string
		for _, v := range azIDsSet.List() {
			azIDList = append(azIDList, v.(string))
		}
		slices.Sort(azIDList)

		if err := validateAvailabilityZoneIDs(ctx, scyllaClient, cloudAccountID, mr.ID, azIDList); err != nil {
			return diag.FromErr(err)
		}

		clusterCreateRequest.AvailabilityZoneIDs = azIDList
	}

	if !versionOK {
		clusterCreateRequest.ScyllaVersionID = scyllaClient.Meta.ScyllaVersions.DefaultScyllaVersionID
	} else if mv := scyllaClient.Meta.VersionByName(version.(string)); mv != nil {
		clusterCreateRequest.ScyllaVersionID = mv.VersionID
	} else {
		return diag.Errorf(`unrecognized value %q for "scylla_version" attribute`, version)
	}

	cr, err := scyllaClient.CreateCluster(ctx, clusterCreateRequest)
	if err != nil {
		return diag.Errorf("failed to create a cluster request: %s", err)
	}

	if err := WaitForClusterRequestID(ctx, scyllaClient, cr.ID); err != nil {
		return diag.Errorf("failed to wait for request %d creating cluster %d: %s", cr.ID, cr.ClusterID, err)
	}

	cluster, err := scyllaClient.GetCluster(ctx, cr.ClusterID)
	if err != nil {
		return diag.Errorf("failed to read cluster %d: %s", cr.ClusterID, err)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	instanceExternalID := ""
	if cluster.Datacenter.InstanceID != 0 {
		i := cloudProvider.InstanceByIDFromInstances(cluster.Datacenter.InstanceID, instances)
		if i == nil {
			return diag.Errorf("unexpected instance ID for cluster %d: %d", cluster.ID, cluster.Datacenter.InstanceID)
		}
		instanceExternalID = i.ExternalID
	}

	err = setClusterKVs(d, cluster, cloudProvider.CloudProvider.Name, instanceExternalID)
	if err != nil {
		return diag.Errorf("failed to set cluster values for cluster %d: %s", cluster.ID, err)
	}

	d.SetId(strconv.Itoa(int(cr.ClusterID)))
	_ = d.Set("request_id", cr.ID)

	return nil
}

func resourceClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Info(ctx, "Read resourceClusterRead")
	scyllaClient := meta.(*scylla.Client)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	reqs, err := scyllaClient.ListClusterRequest(
		ctx,
		clusterID,
		scylla.ListClusterRequestParams{Type: "CREATE_CLUSTER"},
	)
	switch {
	case scylla.IsDeletedErr(err):
		_ = d.Set("status", "DELETED")
		return nil
	case err != nil:
		return diag.Errorf("failed to list cluster requests for cluster %d: %s", clusterID, err)
	case len(reqs) != 1:
		return diag.Errorf("unexpected number of cluster requests; expected 1, got %d: %+v", len(reqs), reqs)
	}
	_ = d.Set("request_id", reqs[0].ID)

	if reqs[0].Status != "COMPLETED" {
		if err := WaitForClusterRequestID(ctx, scyllaClient, reqs[0].ID); err != nil {
			return diag.Errorf("failed to wait for cluster request %d: %s", reqs[0].ID, err)
		}
	}

	cluster, err := scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("failed to read cluster %d: %s", clusterID, err)
	}

	p := scyllaClient.Meta.ProviderByID(cluster.CloudProviderID)
	if p == nil {
		return diag.Errorf("unexpected cloud provider %d for cluster %d", cluster.CloudProviderID, cluster.ID)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	var instanceExternalID string
	if cluster.Datacenter.InstanceID != 0 {
		instances, err := scyllaClient.ListCloudProviderInstancesPerRegion(ctx, cluster.CloudProviderID, cluster.Region.ID)
		if err != nil {
			return diag.Errorf("failed to list cloud provider instances for region %q: %s", cluster.Region.ExternalID, err)
		}

		i := p.InstanceByIDFromInstances(cluster.Datacenter.InstanceID, instances)
		if i == nil {
			return diag.Errorf("unexpected instance ID for cluster %d: %d", cluster.ID, cluster.Datacenter.InstanceID)
		}
		instanceExternalID = i.ExternalID
	}

	err = setClusterKVs(d, cluster, p.CloudProvider.Name, instanceExternalID)
	if err != nil {
		return diag.Errorf("failed to set cluster values for cluster %d: %s", cluster.ID, err)
	}

	return nil
}

func setClusterKVs(d *schema.ResourceData, cluster *model.Cluster, providerName, instanceExternalID string) error {
	_ = d.Set("cluster_id", cluster.ID)
	_ = d.Set("name", cluster.ClusterName)
	_ = d.Set("cloud", providerName)
	_ = d.Set("region", cluster.Region.ExternalID)

	nodeCount := len(model.NodesByStatus(cluster.Nodes, "ACTIVE"))
	_ = d.Set("node_count", nodeCount)

	if clusterUsesScaling(cluster) {
		_ = d.Set("min_nodes", nil)
		_ = d.Set("node_type", nil)
	} else if minNodes, ok := d.GetOk("min_nodes"); !ok {
		_ = d.Set("min_nodes", nodeCount)
	} else if minNodes.(int) > nodeCount {
		// If the cluster was scaled in outside of the Terraform,
		// set min_nodes to its new value. It should result in
		// scale out as it will diverge from what's in the .tf
		// file, which is a true desired state.
		//
		// This covers the following scenario:
		//  - create a cluster using TF with min_nodes = 6,
		//  - scale-in using API
		//  - run "tf apply"
		// Expectations: min_nodes should be updated and the apply should result in scale-out.
		_ = d.Set("min_nodes", nodeCount)
	}

	_ = d.Set("user_api_interface", cluster.UserAPIInterface)
	if !clusterUsesScaling(cluster) {
		_ = d.Set("node_type", instanceExternalID)
	}
	_ = d.Set("node_dns_names", model.NodesDNSNames(cluster.Nodes))
	_ = d.Set("node_private_ips", model.NodesPrivateIPs(cluster.Nodes))
	_ = d.Set("cidr_block", cluster.Datacenter.CIDRBlock)
	_ = d.Set("scylla_version", cluster.ScyllaVersion.Version)
	_ = d.Set("enable_vpc_peering", !strings.EqualFold(cluster.BroadcastType, "PUBLIC"))
	_ = d.Set("enable_dns", cluster.DNS)
	_ = d.Set("datacenter", cluster.Datacenter.Name)
	_ = d.Set("status", cluster.Status)

	if cluster.UserAPIInterface == "ALTERNATOR" {
		_ = d.Set("alternator_write_isolation", cluster.AlternatorWriteIsolation)
	}

	if id := cluster.Datacenter.AccountCloudProviderCredentialID; id >= 1000 {
		_ = d.Set("byoa_id", id)
	}

	if cluster.Instance != nil {
		_ = d.Set("node_disk_size", cluster.Instance.TotalStorage)
	}

	azIDs := cluster.Datacenter.AvailabilityZoneIDs()
	if azIDs == nil {
		// Prevent stale data in case the new value is empty or missing.
		azIDs = []string{}
	}
	_ = d.Set("availability_zone_ids", azIDs)

	return nil
}

func resourceClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Info(context.Background(), "Read resourceClusterUpdate")
	scyllaClient := meta.(*scylla.Client)

	// Currently, only min_nodes is updatable.
	if !d.HasChange("min_nodes") {
		return nil
	}

	// There are three scenarios:
	// - scale-out: newMinNodes > oldMinNodes
	// - scale-in: newMinNodes < oldMinNodes
	// - no-op: newMinNodes == oldMinNodes
	//
	// The no-op case is already handled above by checking `d.HasChange()`.
	//
	// Scale-out is easy: we just request more nodes.
	//
	// Scale-in is more complicated. It may happen that it's currently
	// not possible, because after the scale-in there would be not enough
	// disk space. Such an update should fail, meaning that the value
	// of min_nodes should not be changed. A user can try again later.
	//
	// Note that it's not possible to update min_nodes and defer
	// scale-in until later, e.g., when there is enough disk space,
	// because min_nodes controls the resize API behavior rather
	// than controlling the desired state. If such a behavior is needed,
	// please consider X Cloud. More:
	// https://www.scylladb.com/product/scylladb-xcloud/

	oldMinNodesI, newMinNodesI := d.GetChange("min_nodes")
	oldMinNodes, newMinNodes := oldMinNodesI.(int), newMinNodesI.(int)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	tflog.Debug(ctx, "Updating cluster min_nodes", map[string]interface{}{
		"cluster_id": clusterID,
		"old":        oldMinNodes,
		"new":        newMinNodes,
	})

	cluster, err := scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("failed to get the cluster with ID %d: %s", clusterID, err)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	// Resize will fail if there is any ongoing cluster request.
	if err := WaitForNoInProgressRequests(ctx, scyllaClient, cluster.ID); err != nil {
		return diag.Errorf("failed waiting for no in-progress cluster requests for cluster %d: %s", cluster.ID, err)
	}

	// Re-fetch the cluster to get the current node count after waiting,
	// as it may have changed during the wait period.
	cluster, err = scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("failed to get the cluster with ID %d: %s", clusterID, err)
	}

	curNodesCount := len(model.NodesByStatus(cluster.Nodes, "ACTIVE"))

	if newMinNodes == curNodesCount {
		tflog.Debug(ctx, "Current number of nodes equals min_nodes; return", map[string]interface{}{
			"cluster_id":      clusterID,
			"cur_nodes_count": curNodesCount,
			"new_min_nodes":   newMinNodes,
		})
		return resourceClusterRead(ctx, d, meta)
	}

	tflog.Debug(ctx, "min_nodes is different than current number of nodes; proceed with resize", map[string]interface{}{
		"cluster_id":      clusterID,
		"cur_nodes_count": curNodesCount,
		"new_min_nodes":   newMinNodes,
	})

	resizeRequest, err := scyllaClient.ResizeCluster(
		ctx,
		cluster.ID,
		cluster.Datacenter.ID,
		cluster.Datacenter.InstanceID,
		newMinNodes,
	)
	if err != nil {
		return diag.Errorf("error resizing cluster: %s", err)
	}

	if err := WaitForClusterRequestID(ctx, scyllaClient, resizeRequest.ID); err != nil {
		return diag.Errorf(
			"failed waiting for the cluster resize with ID %d for the cluster %d: %s",
			resizeRequest.ID, cluster.ID, err,
		)
	}

	return resourceClusterRead(ctx, d, meta)
}

func resourceClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Info(context.Background(), "Read resourceClusterDelete")
	c := meta.(*scylla.Client)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	name, ok := d.GetOk("name")
	if !ok {
		return diag.Errorf("failed to read the cluster name from the resource")
	}

	r, err := c.DeleteCluster(ctx, clusterID, name.(string))
	if err != nil {
		if scylla.IsDeletedErr(err) {
			return nil // cluster was already deleted
		}
		return diag.Errorf("failed to delete the cluster: %s", err)
	}

	if !strings.EqualFold(r.Status, "QUEUED") && !strings.EqualFold(r.Status, "IN_PROGRESS") && !strings.EqualFold(r.Status, "COMPLETED") {
		return diag.Errorf(
			"delete cluster returned unknown status %q for the cluster request %d for the cluster %d",
			r.Status, r.ID, clusterID,
		)
	}

	return nil
}

// WaitForClusterRequestID returns only after the cluster request is completed or failed.
func WaitForClusterRequestID(ctx context.Context, c *scylla.Client, requestID int64) error {
	t := time.NewTicker(clusterPollInterval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			r, err := c.GetClusterRequest(ctx, requestID)
			if err != nil {
				return fmt.Errorf("failed to get cluster request with ID %d: %w", requestID, err)
			}

			if strings.EqualFold(r.Status, "COMPLETED") {
				return nil
			}
			if strings.EqualFold(r.Status, "FAILED") {
				return fmt.Errorf("cluster request ID %d failed", r.ID)
			}
			if strings.EqualFold(r.Status, "QUEUED") || strings.EqualFold(r.Status, "IN_PROGRESS") {
				continue
			}

			return fmt.Errorf("unknown cluster request status: %s", r.Status)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForNoInProgressRequests waits until there are no requests in progress.
func WaitForNoInProgressRequests(ctx context.Context, c *scylla.Client, clusterID int64) error {
	t := time.NewTicker(clusterPollInterval)
	defer t.Stop()

	checkAllClear := func() (bool, error) {
		for _, status := range []string{"IN_PROGRESS"} {
			reqs, err := c.ListClusterRequest(
				ctx,
				clusterID,
				scylla.ListClusterRequestParams{Status: status},
			)
			if err != nil {
				return false, err
			}
			if len(reqs) > 0 {
				return false, nil
			}
		}
		return true, nil
	}

	// Check immediately before waiting for the first tick.
	allClear, err := checkAllClear()
	if err != nil {
		return err
	}
	if allClear {
		return nil
	}

	for {
		select {
		case <-t.C:
			allClear, err := checkAllClear()
			if err != nil {
				return err
			}
			if allClear {
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func parseClusterID(d *schema.ResourceData) (int64, diag.Diagnostics) {
	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return 0, diag.Errorf("failed to parse a cluster ID %q: %s", d.Id(), err)
	}
	return clusterID, nil
}

// validateAvailabilityZoneIDs validates that the provided AZ IDs are valid for the given region.
func validateAvailabilityZoneIDs(ctx context.Context, c *scylla.Client, cloudAccountID, regionID int64, azIDs []string) error {
	if l := len(azIDs); l < 1 || l > 3 {
		return fmt.Errorf("at least 1 and at most 3 availability zone IDs are required, got %d", l)
	}

	// Check for duplicate AZ IDs.
	seen := make(map[string]struct{}, len(azIDs))
	var duplicates []string
	for _, azID := range azIDs {
		if _, ok := seen[azID]; ok {
			duplicates = append(duplicates, azID)
		} else {
			seen[azID] = struct{}{}
		}
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate availability zone IDs are not allowed: %v", duplicates)
	}

	// Validate available AZ IDs.
	availableAZs, err := c.ListAvailabilityZoneIDs(ctx, cloudAccountID, regionID)
	if err != nil {
		return fmt.Errorf("failed to list availability zones for region: %w", err)
	}

	availableSet := make(map[string]struct{}, len(availableAZs))
	for _, az := range availableAZs {
		availableSet[az] = struct{}{}
	}

	var invalidAZs []string
	for _, azID := range azIDs {
		if _, ok := availableSet[azID]; !ok {
			invalidAZs = append(invalidAZs, azID)
		}
	}

	if len(invalidAZs) > 0 {
		return fmt.Errorf(
			"invalid availability zone IDs %v; available AZ IDs for this region are: %v",
			invalidAZs,
			availableAZs,
		)
	}

	return nil
}
