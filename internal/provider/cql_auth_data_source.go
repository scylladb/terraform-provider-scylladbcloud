package provider

import (
	"fmt"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DataSourceCQLAuth() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCQLAuthRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(20 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID",
				Required:    true,
				Type:        schema.TypeInt,
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

func dataSourceCQLAuthRead(d *schema.ResourceData, meta interface{}) error {
	var (
		c         = meta.(*scylla.Client)
		clusterID = d.Get("cluster_id").(int)
	)

	conn, err := c.Connect(int64(clusterID))
	if err != nil {
		return fmt.Errorf("error reading connection details: %w", err)
	}

	d.Set("username", conn.Username)
	d.Set("password", conn.Password)
	d.Set("seeds", conn.Seeds)

	return nil
}
