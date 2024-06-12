package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	stackTimeout = 60 * time.Second
)

func ResourceStack() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceStackCreate,
		ReadContext:   resourceStackRead,
		UpdateContext: resourceStackUpdate,
		DeleteContext: resourceStackDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(stackTimeout),
			Update: schema.DefaultTimeout(stackTimeout),
			Delete: schema.DefaultTimeout(stackTimeout),
		},

		Schema: map[string]*schema.Schema{
			"attributes": {
				Description: "List of managed resources",
				Required:    true,
				Type:        schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceStackCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c = meta.(*scylla.Client)
		s = &model.Stack{
			RequestType:        "Create",
			ResourceProperties: d.Get("attributes").(map[string]interface{}),
		}
	)

	id, err := sendStack(ctx, c, s)
	if err != nil {
		return diag.Errorf("failed to create stack: %s", err)
	}

	d.SetId(id)

	return nil
}

func resourceStackRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceStackUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c = meta.(*scylla.Client)
		s = &model.Stack{
			RequestType:        "Update",
			ResourceProperties: d.Get("attributes").(map[string]interface{}),
		}
	)

	_, err := sendStack(ctx, c, s)
	if err != nil {
		return diag.Errorf("failed to update stack: %s", err)
	}

	return nil
}

func resourceStackDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		c = meta.(*scylla.Client)
		s = &model.Stack{
			RequestType:        "Delete",
			ResourceProperties: d.Get("attributes").(map[string]interface{}),
		}
	)

	_, err := sendStack(ctx, c, s)
	if err != nil {
		return diag.Errorf("failed to delete stack: %s", err)
	}

	return nil
}

func sendStack(ctx context.Context, c *scylla.Client, s *model.Stack) (string, error) {
	auth := strings.Split(c.Token, ":")

	if len(auth) != 2 {
		return "", errors.New("invalid token format")
	}

	req := c.V2.Request(ctx, "POST", s, "/")

	req.Header.Set("X-Scylla-Cloud-Stack-Flavor", "tf")

	if err := c.V2.BasicSign(req, auth[0], []byte(auth[1])); err != nil {
		return "", fmt.Errorf("failed to sign request: %w", err)
	}

	if _, err := c.V2.Do(req, s); err != nil {
		return "", fmt.Errorf("failed to create stack: %w", err)
	}

	return auth[0], nil
}
