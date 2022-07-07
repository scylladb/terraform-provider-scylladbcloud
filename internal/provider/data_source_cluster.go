package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = clusterDataSourceType{}
var _ tfsdk.DataSource = clusterDataSource{}

type clusterDataSourceType struct{}

func (t clusterDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {

	clusterSchema := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":                          types.Int64Type,
			"name":                        types.StringType,
			"cluster_name_on_config_file": types.StringType,
			"status":                      types.StringType,
			"cloud_provider_id":           types.Int64Type,
			"replication_factor":          types.Int64Type,
			"broadcast_type":              types.StringType,
			"scylla_version_id":           types.Int64Type,
			"scylla_version":              types.StringType,
			"dc": types.ListType{
				ElemType: types.ObjectType{
					// TODO if the same schema is used in some other data source or resource,
					// extract this to a variable.
					AttrTypes: map[string]attr.Type{
						"id":                                   types.Int64Type,
						"cluster_id":                           types.Int64Type,
						"cloud_provider_id":                    types.Int64Type,
						"cloud_provider_region_id":             types.Int64Type,
						"replication_factor":                   types.Int64Type,
						"ipv4_cidr":                            types.StringType,
						"account_cloud_provider_credential_id": types.Int64Type,
						"status":                               types.StringType,
						"name":                                 types.StringType,
						"management_network":                   types.StringType,
						"instance_type_id":                     types.Int64Type,
						"client_connection": types.ListType{
							ElemType: types.StringType,
						},
					},
				},
			},
			"grafana_url":                  types.StringType,
			"grafana_root_url":             types.StringType,
			"backoffice_grafana_url":       types.StringType,
			"backoffice_prometheus_url":    types.StringType,
			"backoffice_alert_manager_url": types.StringType,
			"encryption_mode":              types.StringType,
			"user_api_interface":           types.StringType,
			"pricing_model":                types.Int64Type,
			"max_allowed_cidr_range":       types.Int64Type,
			"created_at":                   types.StringType,
			"dns":                          types.BoolType,
			"prom_proxy_enabled":           types.BoolType,
		},
	}

	return tfsdk.Schema{
		MarkdownDescription: "Clusters data source",

		Attributes: map[string]tfsdk.Attribute{
			"account_id": {
				MarkdownDescription: "Account id",
				Required:            true,
				Type:                types.Int64Type,
			},
			"cluster_id": {
				MarkdownDescription: "Cluster id",
				Optional:            true,
				Type:                types.Int64Type,
			},
			"name": {
				MarkdownDescription: "Cluster name",
				Optional:            true,
				Type:                types.StringType,
			},
			"cluster": {
				MarkdownDescription: "Matched cluster (if any), if both cluster_id and name are specified, both are used to match a cluster",
				Computed:            true,
				Type:                clusterSchema,
			},
			"all": {
				MarkdownDescription: "List of all clusters belonging to the account",
				Computed:            true,
				Type:                types.ListType{ElemType: clusterSchema},
			},
		},
	}, nil
}

func (t clusterDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return clusterDataSource{provider: provider}, diags
}

type clusterDataSourceData struct {
	ProviderId types.Int64   `tfsdk:"provider_id"`
	All        types.MapType `tfsdk:"all"`
}

type clusterDataSource struct {
	provider provider
}

func (d clusterDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data clusterDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
