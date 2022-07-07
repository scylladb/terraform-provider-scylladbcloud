package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = regionDataSourceType{}
var _ tfsdk.DataSource = regionDataSource{}

type regionDataSourceType struct{}

func (t regionDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Provider regions data source",

		Attributes: map[string]tfsdk.Attribute{
			"provider_id": {
				MarkdownDescription: "ID of the cloud provider",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "Map of all regions, where the key is the region code name (eg. us-east-1)",
				Computed:            true,
				Type: types.MapType{
					ElemType: types.ObjectType{
						AttrTypes: map[string]attr.Type{
							// TODO do we want to include the prices here?
							"id":                               types.Int64Type,
							"cloud_provider_id":                types.Int64Type,
							"name":                             types.StringType,
							"full_name":                        types.StringType,
							"external_id":                      types.StringType,
							"multi_region_external_id":         types.StringType,
							"dc_name":                          types.StringType,
							"backup_storage_gb_cost":           types.StringType,
							"traffic_same_region_in_gb_cost":   types.StringType,
							"traffic_same_region_out_gb_cost":  types.StringType,
							"traffic_cross_region_out_gb_cost": types.StringType,
							"traffic_internet_out_gb_cost":     types.StringType,
							"continent":                        types.StringType,
						},
					},
				},
			},
		},
	}, nil
}

func (t regionDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return regionDataSource{provider: provider}, diags
}

type regionDataSourceData struct {
	ProviderId types.Int64   `tfsdk:"provider_id"`
	All        types.MapType `tfsdk:"all"`
}

type regionDataSource struct {
	provider provider
}

func (d regionDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data regionDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
