package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = providerInstanceDataSourceType{}
var _ tfsdk.DataSource = providerInstanceDataSource{}

type providerInstanceDataSourceType struct{}

var instanceObjectAttrTypes = map[string]attr.Type{
	"id":                              types.Int64Type,
	"cloud_provider_id":               types.Int64Type,
	"name":                            types.StringType,
	"external_name":                   types.StringType,
	"memory_mb":                       types.StringType,
	"local_disk_count":                types.Int64Type,
	"local_storage_total_gb":          types.Int64Type,
	"cpu_core_count":                  types.Int64Type,
	"network_mbps":                    types.Int64Type,
	"external_storage_network_mbps":   types.Int64Type,
	"environment":                     types.StringType,
	"display_order":                   types.Int64Type,
	"network_speed_description":       types.StringType,
	"license_cost_on_demand_per_hour": types.StringType,
	"free_tier_hours":                 types.Int64Type,
	"instance_family":                 types.StringType,
	"group_default":                   types.BoolType,
	"cost_per_hour":                   types.StringType,
	"subscription_cost_hourly":        types.StringType,
	"subscription_cost_monthly":       types.StringType,
	"subscription_cost_yearly":        types.StringType,
}

func (t providerInstanceDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Provider instances data source",

		Attributes: map[string]tfsdk.Attribute{
			"provider_id": {
				MarkdownDescription: "ID of the cloud provider",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "Map of all instances, where the key is the instance code name (eg. t3.micro)",
				Computed:            true,
				Type: types.MapType{
					ElemType: types.ObjectType{AttrTypes: instanceObjectAttrTypes},
				},
			},
		},
	}, nil
}

func (t providerInstanceDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return providerInstanceDataSource{provider: provider}, diags
}

type providerInstanceDataSourceData struct {
	ProviderId types.Int64 `tfsdk:"provider_id"`
	All        types.Map   `tfsdk:"all"`
}

type providerInstanceDataSource struct {
	provider provider
}

func (d providerInstanceDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data providerInstanceDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
