package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = vpcPeeringDataSourceType{}
var _ tfsdk.DataSource = vpcPeeringDataSource{}

type vpcPeeringDataSourceType struct{}

var vpcPeeringAttrs = markAttrsAsComputed(map[string]tfsdk.Attribute{
	"id": {
		MarkdownDescription: "ID",
		Type:                types.Int64Type,
	},
	"external_id": {
		MarkdownDescription: "External ID",
		Type:                types.StringType,
	},
	"cluster_dc_id": {
		MarkdownDescription: "Cluster DC ID",
		Type:                types.Int64Type,
	},
	"peer_vpc_ipv4_cidr_list": {
		MarkdownDescription: "Peer VPC IPv4 CIDR list",
		Type:                types.ListType{ElemType: types.StringType},
	},
	"peer_vpc_ipv4_cidr_list_verified": {
		MarkdownDescription: "Peer VPC IPv4 CIDR list verified",
		Type:                types.ListType{ElemType: types.StringType},
	},
	"peer_vpc_region_id": {
		MarkdownDescription: "Peer VPC region ID",
		Type:                types.Int64Type,
	},
	"peer_vpc_external_id": {
		MarkdownDescription: "Peer VPC external ID",
		Type:                types.StringType,
	},
	"peer_owner_external_id": {
		MarkdownDescription: "Peer owner external ID",
		Type:                types.StringType,
	},
	"status": {
		MarkdownDescription: "Status",
		Type:                types.StringType,
	},
	"expires_at": {
		MarkdownDescription: "Expires at",
		Type:                types.StringType,
	},
	"network_name": {
		MarkdownDescription: "Network name",
		Type:                types.StringType,
	},
	"project_id": {
		MarkdownDescription: "Project ID",
		Type:                types.StringType,
	},
})

var vpcPeeringAttrTypes = extractAttrsTypes(vpcPeeringAttrs)

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
					ElemType: types.ObjectType{AttrTypes: vpcPeeringAttrTypes},
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
