package provider

import (
	"context"
	"os"
	"runtime"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/allowlistrule"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/connection"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cqlauth"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/serverless"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/stack"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/vpcpeering"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var defaultEndpoint = "https://api.cloud.scylladb.com"

func envToken() string {
	return os.Getenv("SCYLLADB_CLOUD_TOKEN")
}

func envEndpoint() string {
	return os.Getenv("SCYLLADB_CLOUD_ENDPOINT")
}

func New(context.Context) *schema.Provider {
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     nonempty(envEndpoint(), defaultEndpoint),
				Description: "URL of the Scylla Cloud endpoint.",
			},
			"token": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				Default:   envToken(),
				ValidateDiagFunc: func(v any, _ cty.Path) diag.Diagnostics {
					if tok, ok := v.(string); !ok || tok == "" {
						return diag.Diagnostics{{
							Severity: diag.Error,
							Summary:  "token is required",
							Detail:   "A token must be provided to authenticate with the Scylla Cloud API.",
						}}
					}
					return nil
				},
				Description: "Bearer token used to authenticate with the API.",
			},
			"metadata": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether to preload deployment metadata for the provider.",
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

	return p
}

func configure(ctx context.Context, p *schema.Provider, d *schema.ResourceData) (*scylla.Client, diag.Diagnostics) {
	var (
		endpoint = d.Get("endpoint").(string)
		token    = d.Get("token").(string)
		metadata = d.Get("metadata").(bool)
	)

	c, err := scylla.NewClient(endpoint, token, userAgent(p.TerraformVersion), metadata)
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

func nonempty[T comparable](t ...T) T {
	var zero T
	for _, v := range t {
		if v != zero {
			return v
		}
	}
	return zero
}
