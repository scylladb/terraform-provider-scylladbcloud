package provider

import (
	"context"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DataSourceCQLAuth() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceCQLAuthRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(20 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID",
				Type:        schema.TypeInt,
				Required:    true,
			},
			"datacenter_id": {
				Description: "Datacenter ID",
				Type:        schema.TypeInt,
				Optional:    true,
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
		c          = meta.(*scylla.Client)
		clusterID  = int64(d.Get("cluster_id").(int))
		dcID, dcOK = d.GetOk("datacenter_id")
		dns        = d.Get("dns").(bool)
	)

	conn, err := c.Connect(ctx, clusterID)
	if err != nil {
		return diag.Errorf("error reading connection details: %s", err)
	}

	if len(conn.Datacenters) == 0 {
		return diag.Errorf("error reading datacenter connections: not found")
	}

	var dc *model.DatacenterConnection

	if dcOK {
		datacenters, err := c.ListDataCenters(ctx, clusterID)
		if err != nil {
			return diag.Errorf("error reading datacenters: %s", err)
		}

	lookup:
		for i := range datacenters {
			lhs := &datacenters[i]

			if lhs.ID == int64(dcID.(int)) {
				for j := range conn.Datacenters {
					rhs := &conn.Datacenters[j]

					if strings.EqualFold(lhs.Name, rhs.Name) {
						dc = rhs
						break lookup
					}
				}
			}
		}

		if dc == nil {
			return diag.Errorf("error looking up datacenter: not found")
		}
	} else {
		dc = &conn.Datacenters[0]
	}

	var seeds string

	if dns {
		if len(dc.DNS) == 0 {
			return diag.Errorf("error reading %q datacenter dns: not found", dc.Name)
		}

		seeds = strings.Join(dc.DNS, ",")
	} else {
		if strings.EqualFold(conn.BroadcastType, "PRIVATE") {
			if len(dc.PrivateIP) == 0 {
				return diag.Errorf("error reading %q datacenter private ip: not found", dc.Name)
			}

			seeds = strings.Join(dc.PrivateIP, ",")
		} else {
			if len(dc.PublicIP) == 0 {
				return diag.Errorf("error reading %q datacenter public ip: not found", dc.Name)
			}

			seeds = strings.Join(dc.PublicIP, ",")
		}
	}

	d.SetId(seeds)
	_ = d.Set("username", conn.Credentials.Username)
	_ = d.Set("password", conn.Credentials.Password)
	_ = d.Set("seeds", seeds)

	return nil
}
