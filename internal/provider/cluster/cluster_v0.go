package cluster

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func resourceClusterV0() *schema.Resource {
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
				Required: true,
				ForceNew: true,
				Type:     schema.TypeInt,
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
		},
	}
}
