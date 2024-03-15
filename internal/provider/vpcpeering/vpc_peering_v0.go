package vpcpeering

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceVPCPeeringV0() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Required: true,
				Type:     schema.TypeInt,
			},
			"datacenter": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"peer_vpc_id": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"peer_cidr_block": {
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"peer_cidr_blocks": {
				Optional: true,
				ForceNew: true,
				Type:     schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"peer_region": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"peer_account_id": {
				Required: true,
				ForceNew: true,
				Type:     schema.TypeString,
			},
			"allow_cql": {
				Optional: true,
				Type:     schema.TypeBool,
				Default:  true,
			},
			"vpc_peering_id": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"connection_id": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"network_link": {
				Computed: true,
				Type:     schema.TypeString,
			},
		},
	}
}
