package cluster

import (
	"context"
	"fmt"
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
	clusterRetryTimeout    = 40 * time.Minute
	clusterDeleteTimeout   = 90 * time.Minute
	clusterRetryDelay      = 5 * time.Second
	clusterRetryMinTimeout = 15 * time.Second
	clusterPollInterval    = 10 * time.Second
)

func ResourceCluster() *schema.Resource {
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

		SchemaVersion: 2,

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
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster id",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"cloud": {
				Description: "Cluster name",
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
				Description: "Minimum number of nodes",
				Required:    true,
				Type:        schema.TypeInt,
				ValidateDiagFunc: func(v interface{}, path cty.Path) diag.Diagnostics {
					value := v.(int)
					if value < 3 {
						return diag.Errorf("min_nodes must be at least 3, got %d", value)
					}
					if value%3 != 0 {
						return diag.Errorf("min_nodes must be divisible by 3, got %d", value)
					}
					return nil
				},
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
				Description: "Instance type of a node",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
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
				Type:        schema.TypeBool,
				// NOTE(rjeczalik): ForceNew is commented out here, otherwise
				// internal provider validate fails due to all the attrs
				// being ForceNew; Scylla Cloud API does not allow for
				// updating existing clusters, thus update the implementation
				// always returns a non-nil error.
				// ForceNew: true,
				Default: true,
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
		},
	}
}

func resourceClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		scyllaClient         = meta.(*scylla.Client)
		clusterCreateRequest = &model.ClusterCreateRequest{
			ClusterName:          d.Get("name").(string),
			BroadcastType:        "PRIVATE",
			ReplicationFactor:    3,
			NumberOfNodes:        int64(d.Get("min_nodes").(int)),
			UserAPIInterface:     d.Get("user_api_interface").(string),
			EnableDNSAssociation: d.Get("enable_dns").(bool),
		}
		cloud                        = d.Get("cloud").(string)
		cidr, cidrOK                 = d.GetOk("cidr_block")
		byoa, byoaOK                 = d.GetOk("byoa_id")
		region                       = d.Get("region").(string)
		nodeType                     = d.Get("node_type").(string)
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

	tflog.Debug(ctx, "Listing cloud provider instances", map[string]interface{}{
		"cloud_provider_id": cloudProvider.CloudProvider.ID,
		"region_id":         mr.ID,
		"instances":         instances,
	})

	var mi *model.CloudProviderInstance
	if nodeDiskSizeOK {
		if mi = cloudProvider.InstanceByNameAndDiskSizeFromInstances(nodeType, nodeDiskSize.(int), instances); mi == nil {
			return diag.Errorf(
				`unrecognized value combination: %q for "node_type" and %d for "node_disk_size" attributes`,
				nodeType,
				nodeDiskSize,
			)
		}
	} else {
		if mi = cloudProvider.InstanceByNameFromInstances(nodeType, instances); mi == nil {
			return diag.Errorf(`unsupported node_type %q in region %s`, nodeType, mr.ExternalID)
		}
	}

	clusterCreateRequest.InstanceID = mi.ID

	if !versionOK {
		clusterCreateRequest.ScyllaVersionID = scyllaClient.Meta.ScyllaVersions.DefaultScyllaVersionID
	} else if mv := scyllaClient.Meta.VersionByName(version.(string)); mv != nil {
		clusterCreateRequest.ScyllaVersionID = mv.VersionID
	} else {
		return diag.Errorf(`unrecognized value %q for "scylla_version" attribute`, version)
	}

	cr, err := scyllaClient.CreateCluster(ctx, clusterCreateRequest)
	if err != nil {
		return diag.Errorf("error creating cluster: %s", err)
	}

	if err := WaitForClusterRequestID(ctx, scyllaClient, cr.ID); err != nil {
		return diag.Errorf("error waiting for cluster: %s", err)
	}

	cluster, err := scyllaClient.GetCluster(ctx, cr.ClusterID)
	if err != nil {
		return diag.Errorf("error reading cluster: %s", err)
	}

	i := cloudProvider.InstanceByIDFromInstances(cluster.Datacenter.InstanceID, instances)
	if i == nil {
		return diag.Errorf("unexpected instance ID: %d", cluster.Datacenter.InstanceID)
	}

	err = setClusterKVs(d, cluster, cloudProvider.CloudProvider.Name, i.ExternalID)
	if err != nil {
		return diag.Errorf("error setting cluster values: %s", err)
	}

	d.SetId(strconv.Itoa(int(cr.ClusterID)))
	_ = d.Set("request_id", cr.ID)

	return nil
}

func resourceClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
		return diag.Errorf("error reading cluster request: %s", err)
	case len(reqs) != 1:
		return diag.Errorf("unexpected number of cluster requests, expected 1, got: %+v", reqs)
	}
	_ = d.Set("request_id", reqs[0].ID)

	if reqs[0].Status != "COMPLETED" {
		if err := WaitForClusterRequestID(ctx, scyllaClient, reqs[0].ID); err != nil {
			return diag.Errorf("error waiting for cluster: %s", err)
		}
	}

	cluster, err := scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("error reading cluster: %s", err)
	}

	p := scyllaClient.Meta.ProviderByID(cluster.CloudProviderID)
	if p == nil {
		return diag.Errorf("unexpected cloud provider ID: %d", cluster.CloudProviderID)
	}

	if n := len(cluster.Datacenters); n > 1 {
		return diag.Errorf("multi-datacenter clusters are not currently supported: %d", n)
	}

	var instanceExternalID string
	if cluster.Datacenter.InstanceID != 0 {
		instances, err := scyllaClient.ListCloudProviderInstancesPerRegion(ctx, cluster.CloudProviderID, cluster.Region.ID)
		if err != nil {
			return diag.Errorf("failed to list cloud provider instances for region %q: %s", cluster.Region.ExternalID, err)
		}

		i := p.InstanceByIDFromInstances(cluster.Datacenter.InstanceID, instances)
		if i == nil {
			return diag.Errorf("unexpected instance ID: %d", cluster.Datacenter.InstanceID)
		}
		instanceExternalID = i.ExternalID
	}
	err = setClusterKVs(d, cluster, p.CloudProvider.Name, instanceExternalID)
	if err != nil {
		return diag.Errorf("error setting cluster values: %s", err)
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

	if minNodes, ok := d.GetOk("min_nodes"); !ok {
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
	_ = d.Set("node_type", instanceExternalID)
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

	return nil
}

func resourceClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
		return diag.Errorf("error reading cluster: %s", err)
	}

	if n := len(cluster.Datacenters); n > 1 {
		return diag.Errorf("multi-datacenter clusters are not currently supported: %d", n)
	}

	// Resize will fail if there is any ongoing cluster request.
	if err := WaitForNoInProgressRequests(ctx, scyllaClient, cluster.ID); err != nil {
		return diag.Errorf("error waiting for no in-progress cluster requests: %s", err)
	}

	curNodesCount := len(model.NodesByStatus(cluster.Nodes, "ACTIVE"))

	if newMinNodes == curNodesCount {
		tflog.Debug(ctx, "Current number of nodes equals min_nodes; return", map[string]interface{}{
			"cluster_id":      clusterID,
			"cur_nodes_count": curNodesCount,
			"new_min_nodes":   newMinNodes,
		})
		return nil
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
		return diag.Errorf("error waiting for cluster resize: %s", err)
	}

	return resourceClusterRead(ctx, d, meta)
}

func resourceClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	name, ok := d.GetOk("name")
	if !ok {
		return diag.Errorf("unable to read cluster name from state file")
	}

	r, err := c.DeleteCluster(ctx, clusterID, name.(string))
	if err != nil {
		if scylla.IsDeletedErr(err) {
			return nil // cluster was already deleted
		}
		return diag.Errorf("error deleting cluster: %s", err)
	}

	if !strings.EqualFold(r.Status, "QUEUED") && !strings.EqualFold(r.Status, "IN_PROGRESS") && !strings.EqualFold(r.Status, "COMPLETED") {
		return diag.Errorf("delete request failure, cluster request id: %d", r.ID)
	}

	return nil
}

func WaitForClusterRequestID(ctx context.Context, c *scylla.Client, requestID int64) error {
	t := time.NewTicker(clusterPollInterval)
	defer t.Stop()

	for range t.C {
		r, err := c.GetClusterRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("error reading cluster request: %w", err)
		}

		if strings.EqualFold(r.Status, "COMPLETED") {
			break
		} else if strings.EqualFold(r.Status, "QUEUED") || strings.EqualFold(r.Status, "IN_PROGRESS") {
			continue
		} else if strings.EqualFold(r.Status, "FAILED") {
			return fmt.Errorf("cluster request failed, cluster request id: %d", r.ID)
		}

		return fmt.Errorf("unrecognized cluster request status: %q", r.Status)
	}

	return nil
}

func WaitForNoInProgressRequests(ctx context.Context, c *scylla.Client, clusterID int64) error {
	t := time.NewTicker(clusterPollInterval)
	defer t.Stop()

	for range t.C {
		reqs, err := c.ListClusterRequest(
			ctx,
			clusterID,
			scylla.ListClusterRequestParams{Status: "IN_PROGRESS"},
		)
		if err != nil {
			return fmt.Errorf("error reading cluster requests: %w", err)
		}

		if len(reqs) == 0 {
			break
		}
	}

	return nil
}

func parseClusterID(d *schema.ResourceData) (int64, diag.Diagnostics) {
	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return 0, diag.Errorf("error reading id=%q: %s", d.Id(), err)
	}
	return clusterID, nil
}
