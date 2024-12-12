package connection

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/schemautils"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

const (
	clusterConnectionRetryTimeout  = 40 * time.Minute
	clusterConnectionDeleteTimeout = 90 * time.Minute
	clusterConnectionRetryDelay    = 5 * time.Second
)

func ResourceClusterConnection() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClusterConnectionCreate,
		ReadContext: func(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
			err := resourceClusterConnectionRead(ctx, data, i)
			if err != nil {
				if errors.Is(err, errNotFound) {
					data.SetId("")
					return nil
				}
				return diag.FromErr(err)
			}
			return nil
		},
		UpdateContext: resourceClusterConnectionUpdate,
		DeleteContext: resourceClusterConnectionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, data *schema.ResourceData, i interface{}) ([]*schema.ResourceData, error) {
				err := resourceClusterConnectionRead(ctx, data, i)
				if err != nil {
					return nil, err
				}
				return []*schema.ResourceData{data}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(clusterConnectionRetryTimeout),
			Update: schema.DefaultTimeout(clusterConnectionRetryTimeout),
			Delete: schema.DefaultTimeout(clusterConnectionDeleteTimeout),
		},

		Schema: map[string]*schema.Schema{
			"id": {
				Description: "Cluster connection ID",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"name": {
				Description: "Cluster Connection Name",
				Optional:    true,
				Type:        schema.TypeString,
			},
			"cluster_id": {
				Description: "Cluster ID",
				Required:    true,
				Type:        schema.TypeInt,
			},
			"datacenter": {
				Description: "Cluster datacenter name",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"cidrlist": {
				Description: "List of CIDRs to route to the cluster connection",
				Required:    true,
				Type:        schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"type": {
				Description: "Connection Type",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"status": {
				Description: "Connection Status",
				Optional:    true,
				Default:     "ACTIVE",
				ValidateDiagFunc: func(i interface{}, s cty.Path) diag.Diagnostics {
					status := i.(string)
					if status != "ACTIVE" && status != "INACTIVE " {
						return diag.Errorf("status must be one of: ACTIVE or INACTIVE")
					}
					return nil
				},
				Type: schema.TypeString,
			},
			"external_id": {
				Description: "ID of the cloud resource that represents connection",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"data": {
				Description: "Connection Data",
				Required:    true,
				Type:        schema.TypeMap,
				ValidateDiagFunc: func(i interface{}, s cty.Path) diag.Diagnostics {
					nonLowerCasedKeys := make([]string, 0)
					for k := range i.(map[string]interface{}) {
						if k != strings.ToLower(k) {
							nonLowerCasedKeys = append(nonLowerCasedKeys, k)
						}
					}
					if len(nonLowerCasedKeys) > 0 {
						return diag.Errorf("data keys must be lowercase: %s", strings.Join(nonLowerCasedKeys, ","))
					}
					return nil
				},
				Elem: &schema.Schema{
					Required: true,
					Type:     schema.TypeString,
				},
			},
		},
	}
}

func resourceClusterConnectionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c                       = meta.(*scylla.Client)
		dcName                  = d.Get("datacenter").(string)
		cidrListVal, cidrListOK = d.GetOk("cidrlist")
		clusterID               = d.Get("cluster_id").(int)
		r                       = &model.ClusterConnectionCreateRequest{
			Name: d.Get("name").(string),
			Data: schemautils.ConvertMapToConcrete[string](d.Get("data").(map[string]interface{})),
			Type: d.Get("type").(string),
		}
		p *scylla.CloudProvider
	)

	dcs, err := c.ListDataCenters(ctx, int64(clusterID))
	if err != nil {
		return diag.Errorf("error reading clusters: %s", err)
	}

	for _, dc := range dcs {
		if strings.EqualFold(dc.Name, dcName) {
			r.ClusterDCID = dc.ID
			p = c.Meta.ProviderByID(dc.CloudProviderID)
			if p == nil {
				return diag.Errorf("unable to find cloud provider with id=%d", dc.CloudProviderID)
			}
			break
		}
	}

	if r.ClusterDCID == 0 {
		return diag.Errorf("unable to find %q datacenter", dcName)
	}

	if !cidrListOK {
		return diag.Errorf(`"cidrlist" is required for %q cloud`, p.CloudProvider.Name)
	}

	if len(cidrListVal.([]any)) == 0 {
		return diag.Errorf(`"cidrlist" cannot be empty`)
	}

	r.CIDRList, err = schemautils.ConvertListToConcrete[string](cidrListVal)
	if err != nil {
		return diag.Errorf(`"cidrlist" must be a list of strings`)
	}

	conn, err := c.CreateClusterConnection(ctx, int64(clusterID), r)
	if err != nil {
		return diag.Errorf("error creating cluster connection: %s", err)
	}
	d.SetId(strconv.FormatInt(conn.ID, 10))
	err = waitForClusterConnection(ctx, c, int64(clusterID), conn.ID, "ACTIVE")
	if err != nil {
		return diag.Errorf("%+v", err)
	}
	conn, err = c.GetClusterConnection(ctx, int64(clusterID), conn.ID)
	if err != nil {
		return diag.Errorf("error reading cluster connection %d: %s", conn.ID, err)
	}
	_ = d.Set("external_id", conn.ExternalID)
	_ = d.Set("status", conn.Status)
	return nil
}

var errNotFound = errors.New("not found")

func resourceClusterConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) error {
	var (
		c         = meta.(*scylla.Client)
		connIDStr = d.Id()
		dc        *model.Datacenter
	)

	if connIDStr == "" {
		return nil
	}

	connectionID, err := strconv.ParseInt(connIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse connection id %q: %w", connIDStr, err)
	}

	clusterID := int64(d.Get("cluster_id").(int))

	cluster, connection, err := findConnection(ctx, c, clusterID, connectionID)
	if err != nil {
		return err
	}
	if connection == nil {
		return errors.Join(errNotFound, errors.New("error reading cluster connection"))
	}
	if cluster == nil {
		return errors.Join(errNotFound, errors.New("error reading cluster"))
	}

	for id := range cluster.Datacenters {
		if cluster.Datacenters[id].ID == connection.ClusterDCID {
			dc = &cluster.Datacenters[id]
			break
		}
	}

	if dc == nil {
		return errors.Join(errNotFound, errors.New("error reading cluster datacenter"))
	}

	_ = d.Set("datacenter", dc.Name)
	_ = d.Set("external_id", connection.ExternalID)
	_ = d.Set("cluster_id", connection.ClusterID)
	_ = d.Set("cidrlist", connection.CIDRList)
	_ = d.Set("name", connection.Name)
	_ = d.Set("data", schemautils.LowerCaseMapKeys(schemautils.ConvertMapFromConcrete(connection.Data)))
	_ = d.Set("type", connection.Type)
	_ = d.Set("status", connection.Status)
	d.SetId(strconv.FormatInt(connection.ID, 10))
	return nil
}

func resourceClusterConnectionUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c           = meta.(*scylla.Client)
		clusterID   = d.Get("cluster_id").(int)
		cidrListVal = d.Get("cidrlist").([]interface{})
	)
	connID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse connection id %q: %s", d.Id(), err)
	}

	cidrlist, err := schemautils.ConvertListToConcrete[string](cidrListVal)
	if err != nil {
		return diag.Errorf("error converting cidrlist: %s", err)
	}

	req := model.ClusterConnectionUpdateRequest{
		Name:     d.Get("name").(string),
		CIDRList: cidrlist,
		Status:   d.Get("status").(string),
	}

	err = c.UpdateClusterConnections(ctx, int64(clusterID), connID, &req)
	if err != nil {
		return diag.Errorf("error updating cluster connection: %s", err)
	}
	err = waitForClusterConnection(ctx, c, int64(clusterID), connID, req.Status)
	if err != nil {
		return diag.Errorf("%+v", err)
	}

	conn, err := c.GetClusterConnection(ctx, int64(clusterID), connID)
	if err != nil {
		return diag.Errorf("error reading cluster connection %d: %s", connID, err)
	}
	_ = d.Set("external_id", conn.ExternalID)
	return nil
}

func resourceClusterConnectionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c         = meta.(*scylla.Client)
		clusterID = d.Get("cluster_id").(int)
		connIDStr = d.Id()
	)

	if connIDStr == "" {
		return nil
	}

	connID, err := strconv.ParseInt(connIDStr, 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse connection id %q: %s", connIDStr, err)
	}

	if err = c.DeleteClusterConnection(ctx, int64(clusterID), connID); err != nil {
		if scylla.IsClusterConnectionDeletedErr(err) {
			return nil // cluster was already deleted
		}
		return diag.Errorf("error deleting cluster connection: %s", err)
	}
	err = waitForClusterConnection(ctx, c, int64(clusterID), connID, "DELETED")
	if err != nil {
		return diag.Errorf("error waiting for cluster connection to become deleted: %s", err)
	}
	return nil
}

func waitForClusterConnection(ctx context.Context, c *scylla.Client, clusterID, connectionID int64, targetStatus string) error {
	stateConf := &retry.StateChangeConf{
		Pending: []string{"PENDING", "INIT", "DELETING"},
		Target:  []string{targetStatus},
		Refresh: func() (interface{}, string, error) {
			conn, err := c.GetClusterConnection(context.Background(), clusterID, connectionID)
			switch {
			case err == nil:
				return 0, conn.Status, nil
			case scylla.IsNotFound(err), scylla.IsClusterConnectionDeletedErr(err):
				return 0, "DELETED", nil
			default:
				return nil, "", err
			}
		},
		Delay:   clusterConnectionRetryDelay,
		Timeout: clusterConnectionRetryTimeout,
	}

	_, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return fmt.Errorf("error waiting for cluster connection to become %q: %s", targetStatus, err)
	}
	return nil
}

func findConnection(ctx context.Context, c *scylla.Client, clusterID, connectionID int64) (cluster *model.Cluster, connection *model.ClusterConnection, err error) {
	if clusterID != 0 {
		cluster, err = c.GetCluster(ctx, clusterID)
		if err != nil {
			if scylla.IsNotFound(err) {
				return nil, nil, nil
			}
			return nil, nil, fmt.Errorf("error reading cluster %d: %s", clusterID, err)
		}

		connection, err = c.GetClusterConnection(ctx, cluster.ID, connectionID)
		switch {
		case err == nil:
			return cluster, connection, nil
		case scylla.IsNotFound(err) || scylla.IsClusterConnectionDeletedErr(err):
			return nil, nil, errNotFound
		default:
			return nil, nil, fmt.Errorf("error reading cluster connection %d: %s", connectionID, err)
		}
	}
	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading cluster list: %s", err)
	}

	for i := range clusters {
		cluster = &clusters[i]
		connection, err = c.GetClusterConnection(ctx, cluster.ID, connectionID)
		if err == nil {
			// Repopulate cluster with detailed information
			cluster, err = c.GetCluster(ctx, cluster.ID)
			if err != nil {
				return nil, nil, fmt.Errorf("error reading cluster %d: %s", clusterID, err)
			}
			return cluster, connection, nil
		}
		if !scylla.IsNotFound(err) && !scylla.IsClusterConnectionDeletedErr(err) {
			return nil, nil, fmt.Errorf("error reading cluster connection %d: %s", connectionID, err)
		}
	}
	return nil, nil, errNotFound
}
