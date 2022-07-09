package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = datacenterDataSourceType{}
var _ tfsdk.DataSource = datacenterDataSource{}

type datacenterDataSourceType struct{}

func (t datacenterDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Data center data source",

		Attributes: map[string]tfsdk.Attribute{
			"id": {
				MarkdownDescription: "ID of the provider",
				Optional:            true,
				Computed:            true,
				Type:                types.Int64Type,
			},
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"cloud_provider_id": {
				MarkdownDescription: "ID of the cloud provider",
				Computed:            true,
				Type:                types.Int64Type,
			},
			"cloud_provider_region_id": {
				MarkdownDescription: "ID of the cloud provider region",
				Computed:            true,
				Type:                types.Int64Type,
			},
			"replication_factor": {
				MarkdownDescription: "Replication factor",
				Computed:            true,
				Type:                types.Int64Type,
			},
			"ipv4_cidr": {
				MarkdownDescription: "IPv4 CIDR",
				Computed:            true,
				Type:                types.StringType,
			},
			"account_cloud_provider_credential_id": {
				MarkdownDescription: "ID of the account cloud provider credential",
				Computed:            true,
				Type:                types.Int64Type,
			},
			"status": {
				MarkdownDescription: "Status",
				Computed:            true,
				Type:                types.StringType,
			},
			"name": {
				MarkdownDescription: "Name, e.g. AWS_US_WEST_1",
				Computed:            true,
				Optional:            true,
				Type:                types.StringType,
			},
			"management_network": {
				MarkdownDescription: "Management network",
				Computed:            true,
				Type:                types.StringType,
			},
			"instance_type_id": {
				MarkdownDescription: "ID of the instance type",
				Computed:            true,
				Type:                types.Int64Type,
			},
		},
	}, nil
}

func (t datacenterDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return datacenterDataSource{provider: provider}, diags
}

type datacenterDataSourceData struct {
	Id                               types.Int64  `tfsdk:"id"`
	ClusterId                        types.Int64  `tfsdk:"cluster_id"`
	CloudProviderId                  types.Int64  `tfsdk:"cloud_provider_id"`
	CloudProviderRegionId            types.Int64  `tfsdk:"cloud_provider_region_id"`
	ReplicationFactor                types.Int64  `tfsdk:"replication_factor"`
	Ipv4Cidr                         types.String `tfsdk:"ipv4_cidr"`
	AccountCloudProviderCredentialId types.Int64  `tfsdk:"account_cloud_provider_credential_id"`
	Status                           types.String `tfsdk:"status"`
	Name                             types.String `tfsdk:"name"`
	ManagementNetwork                types.String `tfsdk:"management_network"`
	InstanceTypeId                   types.Int64  `tfsdk:"instance_type_id"`
}

type datacenterDataSource struct {
	provider provider
}

func (d datacenterDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data datacenterDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO implement

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
