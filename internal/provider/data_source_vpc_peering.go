package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = vpcPeeringDataSourceType{}
var _ tfsdk.DataSource = vpcPeeringDataSource{}

type vpcPeeringDataSourceType struct{}

var vpcPeeringObjectAttrTypes = map[string]attr.Type{
	"id":                               types.Int64Type,
	"external_id":                      types.StringType,
	"cluster_dc_id":                    types.Int64Type,
	"peer_vpc_ipv4_cidr_list":          types.ListType{ElemType: types.StringType},
	"peer_vpc_ipv4_cidr_list_verified": types.ListType{ElemType: types.StringType},
	"peer_vpc_region_id":               types.Int64Type,
	"peer_vpc_external_id":             types.StringType,
	"peer_owner_external_id":           types.StringType,
	"status":                           types.StringType,
	"expires_at":                       types.StringType,
	"network_name":                     types.StringType,
	"project_id":                       types.StringType,
}

func (t vpcPeeringDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Cluster's VPC peerings data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "List of all cluster's VPC peerings",
				Computed:            true,
				Type: types.ListType{
					ElemType: types.ObjectType{AttrTypes: vpcPeeringObjectAttrTypes},
				},
			},
		},
	}, nil
}

func (t vpcPeeringDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return vpcPeeringDataSource{provider: provider}, diags
}

type vpcPeeringDataSourceData struct {
	ClusterId types.Int64 `tfsdk:"cluster_id"`
	All       types.Map   `tfsdk:"all"`
}

type vpcPeeringDataSource struct {
	provider provider
}

func (d vpcPeeringDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data vpcPeeringDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
