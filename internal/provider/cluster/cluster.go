package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

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

		SchemaVersion: 1,

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
				Description: "Node count",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeInt,
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
		c = meta.(*scylla.Client)
		r = &model.ClusterCreateRequest{
			ClusterName:          d.Get("name").(string),
			BroadcastType:        "PRIVATE",
			ReplicationFactor:    3,
			NumberOfNodes:        int64(d.Get("node_count").(int)),
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
		r.BroadcastType = "PUBLIC"
	}

	if r.UserAPIInterface == "ALTERNATOR" {
		r.AlternatorWriteIsolation = d.Get("alternator_write_isolation").(string)
	}

	if byoaOK {
		r.AccountCredentialID = int64(byoa.(int))

		if !strings.EqualFold(cloud, "AWS") {
			return diag.Errorf(`setting "byoa_id" attribute is not supported for cloud=%q`, cloud)
		}
	}

	if !cidrOK {
		cidr = "172.31.0.0/16"
		_ = d.Set("cidr_block", cidr)
	}

	p := c.Meta.ProviderByName(cloud)
	if p == nil {
		return diag.Errorf(`unrecognized value %q for "cloud" attribute`, cloud)
	}

	r.CidrBlock = cidr.(string)

	r.CloudProviderID = p.CloudProvider.ID

	if mr := p.RegionByName(region); mr != nil {
		r.RegionID = mr.ID
	} else {
		return diag.Errorf(`unrecognized value %q for "region" attribute`, region)
	}

	var mi *model.CloudProviderInstance
	if nodeDiskSizeOK {
		if mi = p.InstanceByNameAndDiskSize(nodeType, nodeDiskSize.(int)); mi == nil {
			return diag.Errorf(
				`unrecognized value combination: %q for "node_type" and %d for "node_disk_size" attributes`,
				nodeType,
				nodeDiskSize,
			)
		}
	} else {
		if mi = p.InstanceByName(nodeType); mi == nil {
			return diag.Errorf(`unrecognized value %q for "node_type" attribute`, nodeType)
		}
	}

	r.InstanceID = mi.ID

	if !versionOK {
		r.ScyllaVersionID = c.Meta.ScyllaVersions.DefaultScyllaVersionID
	} else if mv := c.Meta.VersionByName(version.(string)); mv != nil {
		r.ScyllaVersionID = mv.VersionID
	} else {
		return diag.Errorf(`unrecognized value %q for "scylla_version" attribute`, version)
	}

	cr, err := c.CreateCluster(ctx, r)
	if err != nil {
		return diag.Errorf("error creating cluster: %s", err)
	}

	if err := WaitForCluster(ctx, c, cr.ID); err != nil {
		return diag.Errorf("error waiting for cluster: %s", err)
	}

	cluster, err := c.GetCluster(ctx, cr.ClusterID)
	if err != nil {
		return diag.Errorf("error reading cluster: %s", err)
	}

	err = setClusterKVs(d, cluster, p)
	if err != nil {
		return diag.Errorf("error setting cluster values: %s", err)
	}

	d.SetId(strconv.Itoa(int(cr.ClusterID)))
	_ = d.Set("request_id", cr.ID)

	return nil
}

func resourceClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("error reading id=%q: %s", d.Id(), err)
	}

	reqs, err := c.ListClusterRequest(ctx, clusterID, "CREATE_CLUSTER")
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
		if err := WaitForCluster(ctx, c, reqs[0].ID); err != nil {
			return diag.Errorf("error waiting for cluster: %s", err)
		}
	}

	cluster, err := c.GetCluster(ctx, clusterID)
	if err != nil {
		return diag.Errorf("error reading cluster: %s", err)
	}

	p := c.Meta.ProviderByID(cluster.CloudProviderID)
	if p == nil {
		return diag.Errorf("unexpected cloud provider ID: %d", cluster.CloudProviderID)
	}

	if n := len(cluster.Datacenters); n > 1 {
		return diag.Errorf("multi-datacenter clusters are not currently supported: %d", n)
	}

	err = setClusterKVs(d, cluster, p)
	if err != nil {
		return diag.Errorf("error setting cluster values: %s", err)
	}

	return nil
}

func setClusterKVs(d *schema.ResourceData, cluster *model.Cluster, p *scylla.CloudProvider) error {
	_ = d.Set("cluster_id", cluster.ID)
	_ = d.Set("name", cluster.ClusterName)
	_ = d.Set("cloud", p.CloudProvider.Name)
	_ = d.Set("region", cluster.Region.ExternalID)
	_ = d.Set("node_count", len(model.NodesByStatus(cluster.Nodes, "ACTIVE")))
	_ = d.Set("user_api_interface", cluster.UserAPIInterface)
	_ = d.Set("node_type", p.InstanceByID(cluster.Datacenter.InstanceID).ExternalID)
	_ = d.Set("node_dns_names", model.NodesDNSNames(cluster.Nodes))
	_ = d.Set("node_private_ips", model.NodesPrivateIPs(cluster.Nodes))
	_ = d.Set("cidr_block", cluster.Datacenter.CIDRBlock)
	_ = d.Set("scylla_version", cluster.ScyllaVersion.Version)
	_ = d.Set("enable_vpc_peering", !strings.EqualFold(cluster.BroadcastType, "PUBLIC"))
	_ = d.Set("enable_dns", cluster.DNS)
	_ = d.Set("datacenter_id", cluster.Datacenter.ID)
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
	// Scylla Cloud API does not support updating a cluster,
	// thus the update always fails
	return diag.Errorf(`updating "scylla_cluster" resource is not supported`)
}

func resourceClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("error reading id=%q: %s", d.Id(), err)
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
		return diag.Errorf("delete request failure: %q", r.UserFriendlyError)
	}

	return nil
}

func WaitForCluster(ctx context.Context, c *scylla.Client, requestID int64) error {
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
			return fmt.Errorf("cluster request failed: %q", r.UserFriendlyError)
		}

		return fmt.Errorf("unrecognized cluster request status: %q", r.Status)
	}

	return nil
}
