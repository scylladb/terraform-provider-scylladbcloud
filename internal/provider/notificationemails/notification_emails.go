package notificationemails

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

// errCodeNoEmails is what the API returns when the account has none set.
const errCodeNoEmails = "010808"

func ResourceNotificationEmails() *schema.Resource {
	return &schema.Resource{
		Description: "The email addresses that receive the account's notifications. One per account; " +
			"destroying it keeps the addresses, since the account must always have at least one.",

		CreateContext: resourceNotificationEmailsSet,
		ReadContext:   resourceNotificationEmailsRead,
		UpdateContext: resourceNotificationEmailsSet,
		DeleteContext: resourceNotificationEmailsDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"emails": {
				Description: "Email addresses; at least one.",
				Required:    true,
				MinItems:    1,
				Type:        schema.TypeSet,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func emails(d *schema.ResourceData) []string {
	out := make([]string, 0)
	for _, e := range d.Get("emails").(*schema.Set).List() {
		out = append(out, e.(string))
	}
	slices.Sort(out)
	return out
}

func resourceNotificationEmailsSet(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	got, err := c.SetAccountNotificationEmails(ctx, emails(d))
	if err != nil {
		return diag.Errorf("failed to set notification emails: %s", err)
	}
	d.SetId(strconv.FormatInt(c.AccountID, 10))
	_ = d.Set("emails", got)
	return nil
}

func resourceNotificationEmailsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	got, err := c.GetAccountNotificationEmails(ctx)
	var apiErr *scylla.APIError
	if errors.As(err, &apiErr) && apiErr.Code == errCodeNoEmails {
		got, err = nil, nil
	}
	if err != nil {
		return diag.Errorf("failed to read notification emails: %s", err)
	}
	if len(got) == 0 {
		d.SetId("")
		return nil
	}
	if d.Id() == "" {
		d.SetId(strconv.FormatInt(c.AccountID, 10))
	}
	_ = d.Set("emails", got)
	return nil
}

func resourceNotificationEmailsDelete(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
	return diag.Diagnostics{{
		Severity: diag.Warning,
		Summary:  "Notification emails were kept",
		Detail:   "The account must always have at least one notification email, so destroying this resource only removes it from the Terraform state.",
	}}
}
