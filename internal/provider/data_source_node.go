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
	Id                          types.Int64  `tfsdk:"ID"`
	ClusterId                   types.Int64  `tfsdk:"cluster_id"`
	CloudProviderId             types.Int64  `tfsdk:"cloud_provider_id"`
	CloudProviderInstanceTypeId types.Int64  `tfsdk:"cloud_provider_instance_type_id"`
	CloudProviderRegionId       types.Int64  `tfsdk:"cloud_provider_region_id"`
	PublicIp                    types.String `tfsdk:"public_ip"`
	PrivateIp                   types.String `tfsdk:"private_ip"`
	ClusterJoinDate             types.String `tfsdk:"cluster_join_date"`
	ServiceId                   types.Int64  `tfsdk:"service_id"`
	ServiceVersionId            types.Int64  `tfsdk:"service_version_id"`
	ServiceVersion              types.String `tfsdk:"service_version"`
	BillingStartDate            types.String `tfsdk:"billing_start_date"`
	Hostname                    types.String `tfsdk:"hostname"`
	ClusterHostId               types.String `tfsdk:"cluster_host_id"`
	Status                      types.String `tfsdk:"status"`
	NodeState                   types.String `tfsdk:"node_state"`
	ClusterDcId                 types.Int64  `tfsdk:"cluster_dc_id"`
	ServerActionId              types.Int64  `tfsdk:"server_action_id"`
	Distribution                types.String `tfsdk:"distribution"`
	Dns                         types.String `tfsdk:"dns"`
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
