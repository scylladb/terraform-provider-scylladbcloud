package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = allowlistDataSourceType{}
var _ tfsdk.DataSource = allowlistDataSource{}

type allowlistDataSourceType struct{}

var allowlistObjectAttrTypes = map[string]attr.Type{
	"id":             types.Int64Type,
	"cluster_id":     types.Int64Type,
	"source_address": types.StringType,
}

func (t allowlistDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Cluster firewall rules data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "List of all firewall rules",
				Computed:            true,
				Type: types.ListType{
					ElemType: types.ObjectType{AttrTypes: allowlistObjectAttrTypes},
				},
			},
		},
	}, nil
}

func (t allowlistDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return allowlistDataSource{provider: provider}, diags
}

type allowlistDataSourceData struct {
	ClusterId types.Int64 `tfsdk:"cluster_id"`
	All       types.Map   `tfsdk:"all"`
}

type allowlistDataSource struct {
	provider provider
}

func (d allowlistDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data allowlistDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
