// Package datasource holds read-only data sources over the public API.
package datasource

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

func computed(t schema.ValueType, description string) *schema.Schema {
	return &schema.Schema{Type: t, Computed: true, Description: description}
}

func computedList(description string, elem map[string]*schema.Schema) *schema.Schema {
	return &schema.Schema{Type: schema.TypeList, Computed: true, Description: description, Elem: &schema.Resource{Schema: elem}}
}

func stringList(description string) *schema.Schema {
	return &schema.Schema{Type: schema.TypeList, Computed: true, Description: description, Elem: &schema.Schema{Type: schema.TypeString}}
}

func DataSourceAccount() *schema.Resource {
	return &schema.Resource{
		Description: "The ScyllaDB Cloud account the provider's token belongs to.",
		ReadContext: dataSourceAccountRead,
		Schema: map[string]*schema.Schema{
			"account_id":          computed(schema.TypeInt, "Account ID."),
			"name":                computed(schema.TypeString, "Account name."),
			"status":              computed(schema.TypeString, "Account status, e.g. ACTIVE."),
			"role":                computed(schema.TypeString, "The caller's role in the account, e.g. ADMIN."),
			"user_id":             computed(schema.TypeInt, "The caller's user ID."),
			"user_account_status": computed(schema.TypeString, "The caller's membership status in the account."),
		},
	}
}

func dataSourceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	a, err := meta.(*scylla.Client).GetDefaultAccount(ctx)
	if err != nil {
		return diag.Errorf("failed to read account: %s", err)
	}
	d.SetId(strconv.FormatInt(a.AccountID, 10))
	_ = d.Set("account_id", a.AccountID)
	_ = d.Set("name", a.Name)
	_ = d.Set("status", a.AccountStatus)
	_ = d.Set("role", a.Role)
	_ = d.Set("user_id", a.UserID)
	_ = d.Set("user_account_status", a.UserAccountStatus)
	return nil
}

func DataSourceAccountCertificate() *schema.Resource {
	return &schema.Resource{
		Description: "The account's CA certificate, used to verify TLS connections to its clusters.",
		ReadContext: dataSourceAccountCertificateRead,
		Schema: map[string]*schema.Schema{
			"format":  computed(schema.TypeString, "Certificate format, e.g. PEM."),
			"content": computed(schema.TypeString, "The certificate."),
		},
	}
}

func dataSourceAccountCertificateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	cert, err := c.GetAccountCertificate(ctx)
	if err != nil {
		return diag.Errorf("failed to read account certificate: %s", err)
	}
	d.SetId(strconv.FormatInt(c.AccountID, 10))
	_ = d.Set("format", cert.Format)
	_ = d.Set("content", cert.Content)
	return nil
}
