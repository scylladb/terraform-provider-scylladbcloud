package serverless

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"sigs.k8s.io/yaml"
)

func DataSourceServerlessBundle() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceServerlessBundleRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(20 * time.Minute),
		},

		DeprecationMessage: "This data source is deprecated and will be removed in one of the future releases of the provider.",

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID",
				Type:        schema.TypeInt,
				Required:    true,
			},
			"connection_bundle": {
				Description: "Connection Bundle",
				Computed:    true,
				Sensitive:   true,
				Type:        schema.TypeString,
			},
		},
	}
}

func dataSourceServerlessBundleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c         = meta.(*scylla.Client)
		clusterID = int64(d.Get("cluster_id").(int))
	)

	bundle, err := c.Bundle(ctx, clusterID)
	if err != nil {
		return diag.Errorf("error reading connection bundle: %s", err)
	}

	var resource struct {
		Kind string `json:"kind"`
	}

	if err := yaml.Unmarshal(bundle, &resource); err != nil {
		return diag.Errorf("error parsing connection bundle: %s", err)
	}

	if want := "CQLConnectionConfig"; !strings.EqualFold(resource.Kind, want) {
		return diag.Errorf("unexpected connection bundle type: got %q, want %q", resource.Kind, want)
	}

	d.SetId(strconv.Itoa(int(clusterID)))
	_ = d.Set("connection_bundle", string(bundle))

	return nil
}
