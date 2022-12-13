package provider

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/scylladb/terraform-provider-scylla/internal/scylla"
	"github.com/scylladb/terraform-provider-scylla/internal/scylla/model"
)

const (
	allowlistRuleRetryTimeout    = 40 * time.Minute
	allowlistRuleDeleteTimeout   = 90 * time.Minute
	allowlistRuleRetryDelay      = 5 * time.Second
	allowlistRuleRetryMinTimeout = 3 * time.Second
)

func ResourceAllowlistRule() *schema.Resource {
	return &schema.Resource{
		Create: resourceAllowlistRuleCreate,
		Read:   resourceAllowlistRuleRead,
		Update: resourceAllowlistRuleUpdate,
		Delete: resourceAllowlistRuleDelete,

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

func resourceAllowlistRuleCreate(d *schema.ResourceData, meta interface{}) error {
	var (
		c         = meta.(*scylla.Client)
		clusterID = d.Get("cluster_id").(int)
		cidrBlock = d.Get("cidr_block").(string)
		rule      *model.AllowedIP
	)

	rules, err := c.CreateAllowlistRule(int64(clusterID), cidrBlock)
	if err != nil {
		return fmt.Errorf("error creating allowlist rule: %w", err)
	}

	for i := range rules {
		r := &rules[i]

		if strings.EqualFold(r.Address, cidrBlock) {
			rule = r
			break
		}
	}

	if rule == nil {
		return fmt.Errorf("unable to find allowlist rule for %q cidr block", cidrBlock)
	}

	d.SetId(strconv.Itoa(int(rule.ID)))
	d.Set("rule_id", rule.ID)

	return nil
}

func resourceAllowlistRuleRead(d *schema.ResourceData, meta interface{}) error {
	var (
		c       = meta.(*scylla.Client)
		cluster *model.Cluster
		rule    *model.AllowedIP
	)

	ruleID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return fmt.Errorf("error reading id=%q: %w", d.Id(), err)
	}

	clusters, err := c.ListClusters()
	if err != nil {
		return fmt.Errorf("error reading cluster list: %w", err)
	}

lookup:
	for i := range clusters {
		cl := &clusters[i]

		rules, err := c.ListAllowlistRules(cl.ID)
		if err != nil {
			return fmt.Errorf("error reading allowlist rules for cluster ID=%d: %w", cl.ID, err)
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
		return fmt.Errorf("unrecognized rule %d", ruleID)
	}

	d.Set("cidr_block", rule.Address)
	d.Set("cluster_id", cluster.ID)

	return nil
}

func resourceAllowlistRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	return fmt.Errorf(`updating "scylla_allowlist_rule" resource is not supported`)
}

func resourceAllowlistRuleDelete(d *schema.ResourceData, meta interface{}) error {
	var (
		c = meta.(*scylla.Client)
	)

	ruleID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return fmt.Errorf("error reading id=%q: %w", d.Id(), err)
	}

	clusterID, ok := d.GetOk("cluster_id")
	if !ok {
		return fmt.Errorf("unable to read cluster ID from state file")
	}

	if err := c.DeleteAllowlistRule(int64(clusterID.(int)), ruleID); err != nil {
		return fmt.Errorf("error deleting allowlist rule: %w", err)
	}

	return nil
}
