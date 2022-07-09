package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = nodeDataSourceType{}
var _ tfsdk.DataSource = nodeDataSource{}

type nodeDataSourceType struct{}

var nodeObjectAttrTypes = map[string]attr.Type{
	"id":                              types.Int64Type,
	"cluster_id":                      types.Int64Type,
	"cloud_provider_id":               types.Int64Type,
	"cloud_provider_instance_type_id": types.Int64Type,
	"cloud_provider_region_id":        types.Int64Type,
	"public_ip":                       types.StringType,
	"private_ip":                      types.StringType,
	"cluster_join_date":               types.StringType,
	"service_id":                      types.Int64Type,
	"service_version_id":              types.Int64Type,
	"service_version":                 types.StringType,
	"billing_start_date":              types.StringType,
	"hostname":                        types.StringType,
	"cluster_host_id":                 types.StringType,
	"status":                          types.StringType,
	"node_state":                      types.StringType,
	"cluster_dc_id":                   types.Int64Type,
	"server_action_id":                types.Int64Type,
	"distribution":                    types.StringType,
	"dns":                             types.StringType,
}

func (t nodeDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Cluster nodes data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "List of all nodes",
				Computed:            true,
				Type: types.ListType{
					ElemType: types.ObjectType{AttrTypes: nodeObjectAttrTypes},
				},
			},
		},
	}, nil
}

func (t nodeDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return nodeDataSource{provider: provider}, diags
}

type nodeDataSourceData struct {
	ProviderId types.Int64 `tfsdk:"provider_id"`
	All        types.Map   `tfsdk:"all"`
}

type nodeDataSource struct {
	provider provider
}

func (d nodeDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data nodeDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
