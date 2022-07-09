package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = vpcDataSourceType{}
var _ tfsdk.DataSource = vpcDataSource{}

type vpcDataSourceType struct{}

var vpcObjectAttrTypes = map[string]attr.Type{
	"id":                       types.Int64Type,
	"cluster_id":               types.Int64Type,
	"cloud_provider_id":        types.Int64Type,
	"cloud_provider_region_id": types.Int64Type,
	"cluster_dc_id":            types.Int64Type,
	"ipv4_cidr":                types.StringType,
}

func (t vpcDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Cluster's VPCs data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "List of all cluster's VPCs (AWS) or subnets (GCP)",
				Computed:            true,
				Type: types.ListType{
					ElemType: types.ObjectType{AttrTypes: vpcObjectAttrTypes},
				},
			},
		},
	}, nil
}

func (t vpcDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return vpcDataSource{provider: provider}, diags
}

type vpcDataSourceData struct {
	ClusterId types.Int64 `tfsdk:"cluster_id"`
	All       types.Map   `tfsdk:"all"`
}

type vpcDataSource struct {
	provider provider
}

func (d vpcDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data vpcDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
