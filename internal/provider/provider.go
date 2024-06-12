package provider

import (
	"context"
	"net/url"
	"runtime"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/allowlistrule"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/connection"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cqlauth"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/serverless"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/stack"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/vpcpeering"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var defaultEndpoint = &url.URL{
	Scheme: "https",
	Host:   "api.cloud.scylladb.com",
}

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
			"scylladbcloud_cql_auth":          cqlauth.DataSourceCQLAuth(),
			"scylladbcloud_serverless_bundle": serverless.DataSourceServerlessBundle(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"scylladbcloud_cluster":            cluster.ResourceCluster(),
			"scylladbcloud_allowlist_rule":     allowlistrule.ResourceAllowlistRule(),
			"scylladbcloud_vpc_peering":        vpcpeering.ResourceVPCPeering(),
			"scylladbcloud_serverless_cluster": serverless.ResourceServerlessCluster(),
			"scylladbcloud_cluster_connection": connection.ResourceClusterConnection(),
			"scylladbcloud_stack":              stack.ResourceStack(),
		},
	}

	p.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return configure(ctx, p, d)
	}

	return p, nil
}

func configure(ctx context.Context, p *schema.Provider, d *schema.ResourceData) (*scylla.Client, diag.Diagnostics) {
	var (
		endpoint = d.Get("endpoint").(string)
		token    = d.Get("token").(string)
	)

	c, err := scylla.NewClient(endpoint, token, userAgent(p.TerraformVersion))
	if err != nil {
		return nil, diag.Errorf("could not create new Scylla client: %s", err)
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
