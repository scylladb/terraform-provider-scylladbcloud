package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
		MarkdownDescription: "ID of the VPC peering",
		Type:                types.Int64Type,
	},
	"external_id": {
		MarkdownDescription: "External ID of the VPC peering",
		Type:                types.StringType,
	},
	"cluster_dc_id": {
		MarkdownDescription: "ID of the cluster",
		Type:                types.Int64Type,
	},
	"peer_vpc_ipv4_cidr_list": {
		MarkdownDescription: "List of IPv4 CIDRs of the peer VPC",
		Type:                types.ListType{ElemType: types.StringType},
	},
	"peer_vpc_ipv4_cidr_list_verified": {
		MarkdownDescription: "List of verified IPv4 CIDRs of the peer VPC",
		Type:                types.ListType{ElemType: types.StringType},
	},
	"peer_vpc_region_id": {
		MarkdownDescription: "ID of the peer VPC region",
		Type:                types.Int64Type,
	},
	"peer_vpc_external_id": {
		MarkdownDescription: "External ID of the peer VPC",
		Type:                types.StringType,
	},
	"peer_owner_external_id": {
		MarkdownDescription: "External ID of the peer owner",
		Type:                types.StringType,
	},
	"status": {
		MarkdownDescription: "Status of the VPC peering",
		Type:                types.StringType,
	},
	"expires_at": {
		MarkdownDescription: "Expiration date of the VPC peering",
		Type:                types.StringType,
	},
	"network_name": {
		MarkdownDescription: "Name of the network",
		Type:                types.StringType,
	},
	"project_id": {
		MarkdownDescription: "ID of the project",
		Type:                types.StringType,
	},
})

var vpcPeeringAttrsTypes = extractAttrsTypes(vpcPeeringAttrs)

func (t vpcPeeringDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "VPC peerings data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "List of all vpc peerings",
				Computed:            true,
				Type: types.ListType{
					ElemType: types.ObjectType{AttrTypes: vpcPeeringAttrsTypes},
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
	ClusterID types.Int64 `tfsdk:"cluster_id"`
	All       types.List  `tfsdk:"all"`
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

	peerings, err := d.provider.client.ListClusterVPCPeerings(data.ClusterID.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cluster's VPC peerings, got error: %s", err))
		return
	}

	wrappedPeerings := make([]attr.Value, 0, len(peerings))
	for _, peering := range peerings {

		cidrList := types.List{Elems: nil, ElemType: types.StringType}
		for _, cidr := range peering.PeerVPCIPv4CIDRList {
			cidrList.Elems = append(cidrList.Elems, types.String{Value: cidr})
		}
		verifiedCidrList := types.List{Elems: nil, ElemType: types.StringType}
		for _, cidr := range peering.PeerVPCIPv4CIDRListVerified {
			verifiedCidrList.Elems = append(verifiedCidrList.Elems, types.String{Value: cidr})
		}

		wrappedPeerings = append(wrappedPeerings, types.Object{
			Attrs: map[string]attr.Value{
				"id":                               types.Int64{Value: peering.ID},
				"external_id":                      types.String{Value: peering.ExternalID},
				"cluster_dc_id":                    types.Int64{Value: peering.ClusterDCID},
				"peer_vpc_ipv4_cidr_list":          cidrList,
				"peer_vpc_ipv4_cidr_list_verified": verifiedCidrList,
				"peer_vpc_region_id":               types.Int64{Value: peering.PeerVPCRegionID},
				"peer_vpc_external_id":             types.String{Value: peering.PeerVPCExternalID},
				"peer_owner_external_id":           types.String{Value: peering.PeerOwnerExternalID},
				"status":                           types.String{Value: peering.Status},
				"expires_at":                       types.String{Value: peering.ExpiresAt},
				"network_name":                     types.String{Value: peering.NetworkName},
				"project_id":                       types.String{Value: peering.ProjectID},
			},
			AttrTypes: vpcPeeringAttrsTypes,
		})
	}

	data.All = types.List{Elems: wrappedPeerings, ElemType: types.ObjectType{AttrTypes: vpcPeeringAttrsTypes}}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
