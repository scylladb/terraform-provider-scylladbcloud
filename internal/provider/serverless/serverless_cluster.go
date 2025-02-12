package serverless

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

const (
	serverlessClusterRetryTimeout    = 40 * time.Minute
	serverlessClusterDeleteTimeout   = 90 * time.Minute
	serverlessClusterRetryDelay      = 5 * time.Second
	serverlessClusterRetryMinTimeout = 15 * time.Second
	serverlessClusterPollInterval    = 10 * time.Second
)

func ResourceServerlessCluster() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServerlessClusterCreate,
		ReadContext:   resourceServerlessClusterRead,
		UpdateContext: resourceServerlessClusterUpdate,
		DeleteContext: resourceServerlessClusterDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(serverlessClusterRetryTimeout),
			Update: schema.DefaultTimeout(serverlessClusterRetryTimeout),
			Delete: schema.DefaultTimeout(serverlessClusterDeleteTimeout),
		},

		DeprecationMessage: "This resource is deprecated and will be removed in one of the future releases of the provider.",

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Serverless cluster id",
				Computed:    true,
				Type:        schema.TypeInt,
				Required:    false,
				Optional:    true,
			},
			"name": {
				Description: "Serverless cluster name",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"free_tier": {
				Description: "Whether a cluster is in a free tier",
				Default:     true,
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
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
			"units": {
				Description: "Processing units",
				Default:     0,
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
			},
			"hours": {
				Description: "Hours for cluster to last",
				Default:     0,
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
			},
		},
	}
}

func resourceServerlessClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		hours    string
		units    int
		err      error
		c        = meta.(*scylla.Client)
		freeTier = d.Get("free_tier").(bool)
	)

	if freeTier {
		units = d.Get("units").(int)
		intHours := d.Get("hours").(int)
		if intHours != 0 {
			hours = strconv.Itoa(intHours) + "h"
		}
	}

	r := &model.ClusterCreateRequest{
		ClusterName:          d.Get("name").(string),
		BroadcastType:        "PUBLIC",
		CidrBlock:            "172.31.0.0/16",
		CloudProviderID:      1,
		AccountCredentialID:  1,
		RegionID:             1,
		ReplicationFactor:    3,
		NumberOfNodes:        3,
		UserAPIInterface:     "CQL",
		InstanceID:           74,
		FreeTier:             freeTier,
		EnableDNSAssociation: d.Get("enable_dns").(bool),
		Provisioning:         "serverless",
		ProcessingUnits:      units,
		Expiration:           hours,
	}

	cr, err := c.CreateCluster(ctx, r)
	if err != nil {
		return diag.Errorf("error creating serverless cluster: %s", err)
	}

	if err := cluster.WaitForCluster(ctx, c, cr.ID); err != nil {
		return diag.Errorf("error waiting for serverless cluster: %s", err)
	}

	cluster, err := c.GetCluster(ctx, cr.ClusterID)
	if err != nil {
		return diag.Errorf("error reading serverless cluster: %s", err)
	}

	d.SetId(strconv.Itoa(int(cluster.ID)))
	_ = d.Set("cluster_id", int(cluster.ID))

	return nil
}

func resourceServerlessClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
		return diag.Errorf("error reading serverless cluster request: %s", err)
	case len(reqs) != 1:
		return diag.Errorf("unexpected number of serverless cluster requests, expected 1, got: %+v", reqs)
	}
	_ = d.Set("request_id", reqs[0].ID)

	if reqs[0].Status != "COMPLETED" {
		if err := cluster.WaitForCluster(ctx, c, reqs[0].ID); err != nil {
			return diag.Errorf("error waiting for serverless cluster: %s", err)
		}
	}

	cluster, err := c.GetCluster(ctx, clusterID)
	if err != nil {
		return diag.Errorf("error reading serverless cluster: %s", err)
	}

	d.SetId(strconv.Itoa(int(cluster.ID)))
	_ = d.Set("cluster_id", int(cluster.ID))

	return nil
}

func resourceServerlessClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Scylla Cloud API does not support updating a serverless cluster,
	// thus the update always fails
	return diag.Errorf(`updating "scylladbcloud_serverless_cluster" resource is not supported`)
}

func resourceServerlessClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("error reading id=%q: %s", d.Id(), err)
	}

	name, ok := d.GetOk("name")
	if !ok {
		return diag.Errorf("unable to read serverless cluster name from state file")
	}

	r, err := c.DeleteCluster(ctx, clusterID, name.(string))
	if err != nil {
		return diag.Errorf("error deleting serverlessCluster: %s", err)
	}

	if !strings.EqualFold(r.Status, "QUEUED") && !strings.EqualFold(r.Status, "IN_PROGRESS") && !strings.EqualFold(r.Status, "COMPLETED") {
		return diag.Errorf("delete request failure, cluster request id: %q", r.ID)
	}

	return nil
}
