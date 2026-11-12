package cqlauth

import (
	"context"
	"encoding/json"
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
	c := meta.(*scylla.Client)
	clusterID, err := stateInt(rawState, "cluster_id")
	if err != nil {
		return nil, err
	}
	dcID, err := stateInt(rawState, "datacenter_id")
	if err != nil {
		return nil, err
	}

	conn, err := c.Connect(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("error reading connection details: %s", err)
	}

	datacenters, err := c.ListDataCenters(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("error reading datacenters: %s", err)
	}

	dc, err := getConnectionDCByID(conn, datacenters, dcID)
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

// stateInt reads a number from raw JSON state, where the SDK decodes it as a
// float64; an absent key reads as 0.
func stateInt(rawState map[string]interface{}, key string) (int64, error) {
	switch v := rawState[key].(type) {
	case nil:
		return 0, nil
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	default:
		return 0, fmt.Errorf("unexpected %s %v of type %T in state", key, v, v)
	}
}

func getConnectionDCByID(conn *model.ClusterConnectionInformation, datacenters []model.Datacenter, dcID int64) (*model.DatacenterConnection, error) {
	if len(conn.Datacenters) == 0 {
		return nil, fmt.Errorf("error reading datacenter connections: not found")
	}

	if dcID == 0 {
		return &conn.Datacenters[0], nil
	}

	for i := range datacenters {
		lhs := &datacenters[i]

		if lhs.ID == dcID {
			for j := range conn.Datacenters {
				rhs := &conn.Datacenters[j]

				if strings.EqualFold(lhs.Name, rhs.Name) {
					return rhs, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("error looking up datacenter: not found")
}
