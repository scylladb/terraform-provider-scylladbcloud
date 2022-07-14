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
	ID                               types.Int64  `tfsdk:"id"`
	ClusterID                        types.Int64  `tfsdk:"cluster_id"`
	CloudProviderID                  types.Int64  `tfsdk:"provider_id"`
	CloudProviderRegionID            types.Int64  `tfsdk:"cloud_provider_region_id"`
	ReplicationFactor                types.Int64  `tfsdk:"replication_factor"`
	Ipv4Cidr                         types.String `tfsdk:"ipv4_cidr"`
	AccountCloudProviderCredentialID types.Int64  `tfsdk:"account_cloud_provider_credential_id"`
	Status                           types.String `tfsdk:"status"`
	Name                             types.String `tfsdk:"name"`
	ManagementNetwork                types.String `tfsdk:"management_network"`
	InstanceTypeID                   types.Int64  `tfsdk:"instance_type_id"`
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

	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError("malformed data", "id or name must be specified")
		return
	}

	dcs, err := d.provider.client.ListDataCenters(data.ClusterID.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cluster's data centers, got error: %s", err))
		return
	}

	found := false
	for _, d := range dcs {
		if (!data.ID.IsNull() && d.ID == data.ID.Value) || (!data.Name.IsNull() && d.Name == data.Name.Value) {
			data.ID = types.Int64{Value: d.ID}
			data.ClusterID = types.Int64{Value: d.ClusterID}
			data.CloudProviderID = types.Int64{Value: d.CloudProviderID}
			data.CloudProviderRegionID = types.Int64{Value: d.CloudProviderRegionID}
			data.ReplicationFactor = types.Int64{Value: d.ReplicationFactor}
			data.Ipv4Cidr = types.String{Value: d.CIDR}
			data.AccountCloudProviderCredentialID = types.Int64{Value: d.AccountCloudProviderCredentialID}
			data.Status = types.String{Value: d.Status}
			data.Name = types.String{Value: d.Name}
			data.ManagementNetwork = types.String{Value: d.ManagementNetwork}
			data.InstanceTypeID = types.Int64{Value: d.InstanceTypeID}

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
