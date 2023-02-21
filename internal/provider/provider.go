package provider

import (
	"context"
	"net/http"
	"net/url"
	"runtime"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/tfcontext"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	defaultEndpoint = &url.URL{
		Scheme: "https",
		Host:   "api.cloud.scylladb.com",
	}
)

func New(_ context.Context) (*schema.Provider, error) {
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     defaultEndpoint.String(),
				Description: "URL of the Scylla Cloud endpoint.",
			},
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Bearer token used to authenticate with the API.",
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"scylladbcloud_cql_auth":          DataSourceCQLAuth(),
			"scylladbcloud_serverless_bundle": DataSourceServerlessBundle(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"scylladbcloud_cluster":            ResourceCluster(),
			"scylladbcloud_allowlist_rule":     ResourceAllowlistRule(),
			"scylladbcloud_vpc_peering":        ResourceVPCPeering(),
			"scylladbcloud_serverless_cluster": ResourceServerlessCluster(),
		},
	}

	p.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return configure(ctx, p, d)
	}

	c := &scylla.Client{
		Headers: make(http.Header),
	}

	c.Headers.Set("Accept", "application/json")

	p.SetMeta(c)

	return p, nil
}

func configure(ctx context.Context, p *schema.Provider, d *schema.ResourceData) (*scylla.Client, diag.Diagnostics) {
	var (
		endpoint = d.Get("endpoint").(string)
		token    = d.Get("token").(string)
		c        = p.Meta().(*scylla.Client)
	)
	ctx = tfcontext.AddProviderInfo(ctx, endpoint)
	var err error
	if c.Endpoint, err = url.Parse(endpoint); err != nil {
		return nil, diag.FromErr(err)
	}

	c.Headers.Set("User-Agent", userAgent(p.TerraformVersion))

	if err := c.Auth(ctx, token); err != nil {
		return nil, diag.FromErr(err)
	}

	return c, nil
}

func userAgent(tfVersion string) string {
	sysinfo := "(" + runtime.GOOS + "/" + runtime.GOARCH + ")"

	if tfVersion != "" {
		return "Terraform/" + tfVersion + "(" + sysinfo + ")"
	}

	return "Terraform/0.11+compatible (" + sysinfo + ")"
}
