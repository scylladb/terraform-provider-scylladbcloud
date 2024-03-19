package cqlauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func dataSourceCQLAuthV0() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"datacenter_id": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"dns": {
				Optional: true,
				Default:  true,
				Type:     schema.TypeBool,
			},
			"seeds": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"username": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"password": {
				Computed:  true,
				Sensitive: true,
				Type:      schema.TypeString,
			},
		},
	}
}

func dataSourceCQLAuthUpgradeFrom0(ctx context.Context, rawState map[string]interface{}, meta interface{}) (map[string]interface{}, error) {
	var (
		c         = meta.(*scylla.Client)
		clusterID = int64(rawState["cluster_id"].(int))
		dcID      = rawState["datacenter_id"].(int)
	)

	conn, err := c.Connect(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("error reading connection details: %s", err)
	}

	datacenters, err := c.ListDataCenters(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("error reading datacenters: %s", err)
	}

	dc, err := getConnectionDCByID(conn, datacenters, int64(dcID))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cluster_id": rawState["cluster_id"],
		"datacenter": dc.Name,
		"dns":        rawState["dns"],
		"seeds":      rawState["seeds"],
		"username":   rawState["username"],
		"password":   rawState["password"],
	}, nil
}

func getConnectionDCByID(conn *model.ClusterConnection, datacenters []model.Datacenter, dcID int64) (*model.DatacenterConnection, error) {
	if len(conn.Datacenters) == 0 {
		return nil, fmt.Errorf("error reading datacenter connections: not found")
	}

	var dc *model.DatacenterConnection

	if dcID == 0 {
		return &conn.Datacenters[0], nil
	}

	for i := range datacenters {
		lhs := &datacenters[i]

		if lhs.ID == dcID {
			for j := range conn.Datacenters {
				rhs := &conn.Datacenters[j]

				if strings.EqualFold(lhs.Name, rhs.Name) {
					return dc, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("error looking up datacenter: not found")
}
