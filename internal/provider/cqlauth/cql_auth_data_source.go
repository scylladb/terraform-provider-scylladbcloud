package cqlauth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func DataSourceCQLAuth() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceCQLAuthRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(20 * time.Minute),
		},

		SchemaVersion: 1,

		StateUpgraders: []schema.StateUpgrader{
			{
				Version: 0,
				Type:    dataSourceCQLAuthV0().CoreConfigSchema().ImpliedType(),
				Upgrade: dataSourceCQLAuthUpgradeFrom0,
			},
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID",
				Type:        schema.TypeInt,
				Required:    true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					if i.(int) < 1 {
						return nil, []error{fmt.Errorf("cluster_id must be greater than 0")}
					}
					return nil, nil
				},
			},
			"datacenter": {
				Description: "Datacenter",
				Type:        schema.TypeString,
				Computed:    true,
				Optional:    true,
			},
			"cluster_name": {
				Description: "Cluster name",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"dns": {
				Description: "Use DNS names for seeds",
				Optional:    true,
				Default:     true,
				Type:        schema.TypeBool,
			},
			"seeds": {
				Description: "Comma-separate seed node addresses",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"username": {
				Description: "CQL username",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"password": {
				Description: "CQL password",
				Computed:    true,
				Sensitive:   true,
				Type:        schema.TypeString,
			},
		},
	}
}

func dataSourceCQLAuthRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c         = meta.(*scylla.Client)
		clusterID = int64(d.Get("cluster_id").(int))
		dcName    = d.Get("datacenter").(string)
		dns       = d.Get("dns").(bool)
	)

	cluster, err := c.GetCluster(ctx, clusterID)
	if err != nil {
		return diag.Errorf("error reading cluster: %s", err)
	}

	conn, err := c.Connect(ctx, clusterID)
	if err != nil {
		return diag.Errorf("error reading connection details: %s", err)
	}

	dc, err := getConnectionDCByName(conn, dcName)
	if err != nil {
		return diag.FromErr(err)
	}

	seeds, err := getSeeds(dns, dc, conn)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(seeds)
	_ = d.Set("cluster_name", cluster.ClusterName)
	_ = d.Set("datacenter", dc.Name)
	_ = d.Set("username", conn.Credentials.Username)
	_ = d.Set("password", conn.Credentials.Password)
	_ = d.Set("seeds", seeds)

	return nil
}

func getSeeds(dns bool, dc *model.DatacenterConnection, conn *model.ClusterConnectionInformation) (string, error) {
	if dns {
		if len(dc.DNS) == 0 {
			return "", fmt.Errorf("error reading %q datacenter dns: not found", dc.Name)
		}

		return strings.Join(dc.DNS, ","), nil
	}
	if strings.EqualFold(conn.BroadcastType, "PRIVATE") {
		if len(dc.PrivateIP) == 0 {
			return "", fmt.Errorf("error reading %q datacenter private ip: not found", dc.Name)
		}

		return strings.Join(dc.PrivateIP, ","), nil
	}
	if len(dc.PublicIP) == 0 {
		return "", fmt.Errorf("error reading %q datacenter public ip: not found", dc.Name)
	}

	return strings.Join(dc.PublicIP, ","), nil
}

func getConnectionDCByName(conn *model.ClusterConnectionInformation, dcName string) (*model.DatacenterConnection, error) {
	if len(conn.Datacenters) == 0 {
		return nil, fmt.Errorf("error reading datacenter connections: not found")
	}

	if dcName == "" {
		return &conn.Datacenters[0], nil
	}

	for _, connDC := range conn.Datacenters {
		if strings.EqualFold(dcName, connDC.Name) {
			return &connDC, nil
		}
	}
	return nil, fmt.Errorf("error looking up datacenter: not found")
}
