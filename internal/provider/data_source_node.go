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
var _ tfsdk.DataSourceType = nodeDataSourceType{}
var _ tfsdk.DataSource = nodeDataSource{}

type nodeDataSourceType struct{}

var nodeAttrs = markAttrsAsComputed(
	map[string]tfsdk.Attribute{
		"id": {
			MarkdownDescription: "ID of the node",
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
		"cloud_provider_instance_type_id": {
			MarkdownDescription: "ID of the cloud provider instance type",
			Type:                types.Int64Type,
		},
		"cloud_provider_region_id": {
			MarkdownDescription: "ID of the cloud provider region",
			Type:                types.Int64Type,
		},
		"public_ip": {
			MarkdownDescription: "Public IP of the node",
			Type:                types.StringType,
		},
		"private_ip": {
			MarkdownDescription: "Private IP of the node",
			Type:                types.StringType,
		},
		"cluster_join_date": {
			MarkdownDescription: "Date when the node joined the cluster",
			Type:                types.StringType,
		},
		"service_id": {
			MarkdownDescription: "ID of the service",
			Type:                types.Int64Type,
		},
		"service_version_id": {
			MarkdownDescription: "ID of the service version",
			Type:                types.Int64Type,
		},
		"service_version": {
			MarkdownDescription: "Version of the service",
			Type:                types.StringType,
		},
		"billing_start_date": {
			MarkdownDescription: "Date when the service was billed",
			Type:                types.StringType,
		},
		"hostname": {
			MarkdownDescription: "Hostname of the node",
			Type:                types.StringType,
		},
		"cluster_host_id": {
			MarkdownDescription: "ID of the cluster host",
			Type:                types.StringType,
		},
		"status": {
			MarkdownDescription: "Status of the node",
			Type:                types.StringType,
		},
		"node_state": {
			MarkdownDescription: "State of the node",
			Type:                types.StringType,
		},
		"cluster_dc_id": {
			MarkdownDescription: "ID of the cluster datacenter",
			Type:                types.Int64Type,
		},
		"server_action_id": {
			MarkdownDescription: "ID of the server action",
			Type:                types.Int64Type,
		},
		"distribution": {
			MarkdownDescription: "Distribution of the node",
			Type:                types.StringType,
		},
		"dns": {
			MarkdownDescription: "DNS of the node",
			Type:                types.StringType,
		},
	},
)

var nodeAttrsTypes = extractAttrsTypes(nodeAttrs)

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
					ElemType: types.ObjectType{AttrTypes: nodeAttrsTypes},
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
	ClusterID types.Int64 `tfsdk:"cluster_id"`
	All       types.List  `tfsdk:"all"`
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

	nodes, err := d.provider.client.ListClusterNodes(data.ClusterID.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cluster's nodes, got error: %s", err))
		return
	}

	wrappedNodes := make([]attr.Value, 0, len(nodes))
	for _, node := range nodes {
		wrappedNodes = append(wrappedNodes, types.Object{
			Attrs: map[string]attr.Value{
				"id":                              types.Int64{Value: node.ID},
				"cluster_id":                      types.Int64{Value: node.ClusterID},
				"provider_id":                     types.Int64{Value: node.CloudProviderID},
				"cloud_provider_instance_type_id": types.Int64{Value: node.CloudProviderInstanceTypeID},
				"cloud_provider_region_id":        types.Int64{Value: node.CloudProviderRegionID},
				"public_ip":                       types.String{Value: node.PublicIP},
				"private_ip":                      types.String{Value: node.PrivateIP},
				"cluster_join_date":               types.String{Value: node.ClusterJoinDate},
				"service_id":                      types.Int64{Value: node.ServiceID},
				"service_version_id":              types.Int64{Value: node.ServiceVersionID},
				"service_version":                 types.String{Value: node.ServiceVersion},
				"billing_start_date":              types.String{Value: node.BillingStartDate},
				"hostname":                        types.String{Value: node.Hostname},
				"cluster_host_id":                 types.String{Value: node.ClusterHostID},
				"status":                          types.String{Value: node.Status},
				"node_state":                      types.String{Value: node.NodeState},
				"cluster_dc_id":                   types.Int64{Value: node.ClusterDcID},
				"server_action_id":                types.Int64{Value: node.ServerActionID},
				"distribution":                    types.String{Value: node.Distribution},
				"dns":                             types.String{Value: node.DNS},
			},
			AttrTypes: nodeAttrsTypes,
		})
	}

	data.All = types.List{Elems: wrappedNodes, ElemType: types.ObjectType{AttrTypes: nodeAttrsTypes}}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
