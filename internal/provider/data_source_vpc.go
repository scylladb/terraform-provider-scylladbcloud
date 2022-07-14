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
var _ tfsdk.DataSourceType = vpcDataSourceType{}
var _ tfsdk.DataSource = vpcDataSource{}

type vpcDataSourceType struct{}

var vpcAttrs = markAttrsAsComputed(
	map[string]tfsdk.Attribute{
		"id": {
			MarkdownDescription: "ID of the VPC",
			Type:                types.Int64Type,
		},
		"cluster_id": {
			MarkdownDescription: "ID of the cluster",
			Type:                types.Int64Type,
		},
		"provider_id": {
			MarkdownDescription: "ID of the cloud provider",
			Type:                types.Int64Type,
		},
		"cloud_provider_region_id": {
			MarkdownDescription: "ID of the cloud provider region",
			Type:                types.Int64Type,
		},
		"cluster_dc_id": {
			MarkdownDescription: "ID of the cluster data center",
			Type:                types.Int64Type,
		},
		"cidr": {
			MarkdownDescription: "IPv4 CIDR of the VPC",
			Type:                types.StringType,
		},
	},
)

var vpcAttrsTypes = extractAttrsTypes(vpcAttrs)

func (t vpcDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Cluster VPCs data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "List of all VPCs",
				Computed:            true,
				Type: types.ListType{
					ElemType: types.ObjectType{AttrTypes: vpcAttrsTypes},
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
	ClusterID types.Int64 `tfsdk:"cluster_id"`
	All       types.List  `tfsdk:"all"`
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

	vpcs, err := d.provider.client.ListClusterVPCs(data.ClusterID.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cluster's VPCs, got error: %s", err))
		return
	}

	wrappedVpcs := make([]attr.Value, 0, len(vpcs))
	for _, vpc := range vpcs {
		wrappedVpcs = append(wrappedVpcs, types.Object{
			Attrs: map[string]attr.Value{
				"id":                       types.Int64{Value: vpc.ID},
				"cluster_id":               types.Int64{Value: vpc.ClusterID},
				"provider_id":              types.Int64{Value: vpc.CloudProviderID},
				"cloud_provider_region_id": types.Int64{Value: vpc.CloudProviderRegionID},
				"cluster_dc_id":            types.Int64{Value: vpc.ClusterDcID},
				"cidr":                     types.String{Value: vpc.CIDR},
			},
			AttrTypes: vpcAttrsTypes,
		})
	}

	data.All = types.List{Elems: wrappedVpcs, ElemType: types.ObjectType{AttrTypes: vpcAttrsTypes}}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
