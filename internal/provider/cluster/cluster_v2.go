package cluster

import (
	"context"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceClusterV2 returns the schema for cluster resource version 2.
// This version does not include the availability_zone_ids field.
func resourceClusterV2() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"cloud": {
				Optional: true,
				ForceNew: true,
				Default:  "AWS",
				Type:     schema.TypeString,
			},
			"name": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"region": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"node_count": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"min_nodes": {
				Required: true,
				Type:     schema.TypeInt,
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
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeInt,
			},
			"user_api_interface": {
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeString,
				Default:  "CQL",
			},
			"alternator_write_isolation": {
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeString,
				Default:  "only_rmw_uses_lwt",
			},
			"node_type": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"node_dns_names": {
				Computed: true,
				Type:     schema.TypeSet,
				Elem:     schema.TypeString,
				Set:      schema.HashString,
			},
			"node_private_ips": {
				Computed: true,
				Type:     schema.TypeSet,
				Elem:     schema.TypeString,
				Set:      schema.HashString,
			},
			"cidr_block": {
				Optional: true,
				Computed: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"scylla_version": {
				Optional: true,
				Computed: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"enable_vpc_peering": {
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeBool,
				Default:  true,
			},
			"enable_dns": {
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeBool,
				Default:  true,
			},
			"request_id": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"datacenter": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"status": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"node_disk_size": {
				ForceNew: true,
				Optional: true,
				Computed: true,
				Type:     schema.TypeInt,
			},
		},
	}
}

// resourceClusterUpgradeV2 migrates state from version 2 to version 3.
// This migration adds the availability_zone_ids field which will be
// populated on the next read from the server.
func resourceClusterUpgradeV2(_ context.Context, rawState map[string]any, _ any) (map[string]any, error) {
	// availability_zone_ids is a new computed+optional field.
	// No migration needed - the field will be populated on the next read.
	return rawState, nil
}
