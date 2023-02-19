package provider

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	allowlistRuleRetryTimeout    = 40 * time.Minute
	allowlistRuleDeleteTimeout   = 90 * time.Minute
	allowlistRuleRetryDelay      = 5 * time.Second
	allowlistRuleRetryMinTimeout = 3 * time.Second
)

func ResourceAllowlistRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAllowlistRuleCreate,
		ReadContext:   resourceAllowlistRuleRead,
		UpdateContext: resourceAllowlistRuleUpdate,
		DeleteContext: resourceAllowlistRuleDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(allowlistRuleRetryTimeout),
			Update: schema.DefaultTimeout(allowlistRuleRetryTimeout),
			Delete: schema.DefaultTimeout(allowlistRuleDeleteTimeout),
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "Cluster ID",
				Required:    true,
				// ForceNew:    true,
				Type: schema.TypeInt,
			},
			"cidr_block": {
				Description: "Allowlisted CIDR block",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"rule_id": {
				Description: "Rule ID",
				Computed:    true,
				Type:        schema.TypeInt,
			},
		},
	}
}

func resourceAllowlistRuleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c         = meta.(*scylla.Client)
		clusterID = d.Get("cluster_id").(int)
		cidrBlock = d.Get("cidr_block").(string)
		rule      *model.AllowedIP
	)

	rules, err := c.CreateAllowlistRule(ctx, int64(clusterID), cidrBlock)
	if err != nil {
		return diag.Errorf("error creating allowlist rule: %s", err)
	}

	for i := range rules {
		r := &rules[i]

		if strings.EqualFold(r.Address, cidrBlock) {
			rule = r
			break
		}
	}

	if rule == nil {
		return diag.Errorf("unable to find allowlist rule for %q cidr block", cidrBlock)
	}

	d.SetId(strconv.Itoa(int(rule.ID)))
	d.Set("rule_id", rule.ID)

	return nil
}

func resourceAllowlistRuleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c       = meta.(*scylla.Client)
		cluster *model.Cluster
		rule    *model.AllowedIP
	)

	ruleID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("error reading id=%q: %s", d.Id(), err)
	}

	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return diag.Errorf("error reading cluster list: %s", err)
	}

lookup:
	for i := range clusters {
		cl := &clusters[i]

		rules, err := c.ListAllowlistRules(ctx, cl.ID)
		if err != nil {
			return diag.Errorf("error reading allowlist rules for cluster ID=%d: %s", cl.ID, err)
		}

		for j := range rules {
			r := &rules[j]

			if r.ID == ruleID {
				cluster = cl
				rule = r
				break lookup
			}
		}
	}

	if rule == nil {
		return diag.Errorf("unrecognized rule %d", ruleID)
	}

	d.Set("cidr_block", rule.Address)
	d.Set("cluster_id", cluster.ID)

	return nil
}

func resourceAllowlistRuleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.Errorf(`updating "scylla_allowlist_rule" resource is not supported`)
}

func resourceAllowlistRuleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c = meta.(*scylla.Client)
	)

	ruleID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("error reading id=%q: %s", d.Id(), err)
	}

	clusterID, ok := d.GetOk("cluster_id")
	if !ok {
		return diag.Errorf("unable to read cluster ID from state file")
	}

	if err := c.DeleteAllowlistRule(ctx, int64(clusterID.(int)), ruleID); err != nil {
		if scylla.IsDeletedErr(err) {
			return nil // cluster was already deleted
		}
		return diag.Errorf("error deleting allowlist rule: %s", err)
	}

	return nil
}
