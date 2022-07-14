package provider

import (
	"context"
	"fmt"
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
				MarkdownDescription: "ID of the data center",
				Optional:            true,
				Computed:            true,
				Type:                types.Int64Type,
			},
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"provider_id": {
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
	CloudProviderId                  types.Int64  `tfsdk:"provider_id"`
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

	if data.Id.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError("malformed data", "id or name must be specified")
		return
	}

	dcs, err := d.provider.client.ListDataCenters(data.ClusterId.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cluster's data centers, got error: %s", err))
		return
	}

	found := false
	for _, d := range dcs {
		if (!data.Id.IsNull() && d.Id == data.Id.Value) || (!data.Name.IsNull() && d.Name == data.Name.Value) {
			data.Id = types.Int64{Value: d.Id}
			data.ClusterId = types.Int64{Value: d.ClusterId}
			data.CloudProviderId = types.Int64{Value: d.CloudProviderId}
			data.CloudProviderRegionId = types.Int64{Value: d.CloudProviderRegionId}
			data.ReplicationFactor = types.Int64{Value: d.ReplicationFactor}
			data.Ipv4Cidr = types.String{Value: d.Ipv4Cidr}
			data.AccountCloudProviderCredentialId = types.Int64{Value: d.AccountCloudProviderCredentialId}
			data.Status = types.String{Value: d.Status}
			data.Name = types.String{Value: d.Name}
			data.ManagementNetwork = types.String{Value: d.ManagementNetwork}
			data.InstanceTypeId = types.Int64{Value: d.InstanceTypeId}

			found = true
			break
		}
	}
	if !found {
		resp.Diagnostics.AddError("Not Found", "No data center matching criteria found")
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
