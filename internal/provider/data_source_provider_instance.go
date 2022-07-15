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
var _ tfsdk.DataSourceType = providerInstanceDataSourceType{}
var _ tfsdk.DataSource = providerInstanceDataSource{}

type providerInstanceDataSourceType struct{}

var instanceAttrs = markAttrsAsComputed(map[string]tfsdk.Attribute{
	"id": {
		MarkdownDescription: "ID of the instance",
		Type:                types.Int64Type,
	},
	"provider_id": {
		MarkdownDescription: "ID of the cloud provider",
		Type:                types.Int64Type,
	},
	"name": {
		MarkdownDescription: "Name of the instance",
		Type:                types.StringType,
	},
	"external_id": {
		MarkdownDescription: "External name of the instance",
		Type:                types.StringType,
	},
	"memory_mb": {
		MarkdownDescription: "Memory in MB",
		Type:                types.Int64Type,
	},
	"local_disk_count": {
		MarkdownDescription: "Number of local disks",
		Type:                types.Int64Type,
	},
	"local_storage_total_gb": {
		MarkdownDescription: "Total local storage in GB",
		Type:                types.Int64Type,
	},
	"cpu_core_count": {
		MarkdownDescription: "Number of CPU cores",
		Type:                types.Int64Type,
	},
	"network_mbps": {
		MarkdownDescription: "Network speed in MB/s",
		Type:                types.Int64Type,
	},
	"external_storage_network_mbps": {
		MarkdownDescription: "External storage network speed in MB/s",
		Type:                types.Int64Type,
	},
	"environment": {
		MarkdownDescription: "Environment",
		Type:                types.StringType,
	},
	"display_order": {
		MarkdownDescription: "Display order",
		Type:                types.Int64Type,
	},
	"network_speed_description": {
		MarkdownDescription: "Network speed description",
		Type:                types.StringType,
	},
	"license_cost_on_demand_per_hour": {
		MarkdownDescription: "License cost on demand per hour",
		Type:                types.StringType,
	},
	"free_tier_hours": {
		MarkdownDescription: "Free tier hours",
		Type:                types.Int64Type,
	},
	"instance_family": {
		MarkdownDescription: "Instance family",
		Type:                types.StringType,
	},
	"group_default": {
		MarkdownDescription: "Is this instance the default for its group",
		Type:                types.BoolType,
	},
	"cost_per_hour": {
		MarkdownDescription: "Cost per hour",
		Type:                types.StringType,
	},
	"subscription_cost_hourly": {
		MarkdownDescription: "Subscription cost hourly",
		Type:                types.StringType,
	},
	"subscription_cost_monthly": {
		MarkdownDescription: "Subscription cost monthly",
		Type:                types.StringType,
	},
	"subscription_cost_yearly": {
		MarkdownDescription: "Subscription cost yearly",
		Type:                types.StringType,
	},
})

var instanceAttrsTypes = extractAttrsTypes(instanceAttrs)

func (t providerInstanceDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Provider instances data source",

		Attributes: map[string]tfsdk.Attribute{
			"provider_id": {
				MarkdownDescription: "ID of the cloud provider",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "Map of all instances, where the key is the instance code name (eg. t3.micro)",
				Computed:            true,
				Type: types.MapType{
					ElemType: types.ObjectType{AttrTypes: instanceAttrsTypes},
				},
			},
		},
	}, nil
}

func (t providerInstanceDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return providerInstanceDataSource{provider: provider}, diags
}

type providerInstanceDataSourceData struct {
	ProviderID types.Int64 `tfsdk:"provider_id"`
	All        types.Map   `tfsdk:"all"`
}

type providerInstanceDataSource struct {
	provider provider
}

func (d providerInstanceDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data providerInstanceDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	instances, err := d.provider.client.ListCloudProviderInstances(data.ProviderID.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cloud provider instances, got error: %s", err))
		return
	}

	instancesByName := make(map[string]attr.Value)
	for _, instance := range instances {
		instancesByName[instance.Name] = types.Object{
			Attrs: map[string]attr.Value{
				"id":                              types.Int64{Value: instance.ID},
				"provider_id":                     types.Int64{Value: instance.CloudProviderID},
				"name":                            types.String{Value: instance.Name},
				"external_id":                     types.String{Value: instance.ExternalID},
				"memory_mb":                       types.Int64{Value: instance.MemoryMB},
				"local_disk_count":                types.Int64{Value: instance.LocalDiskCount},
				"local_storage_total_gb":          types.Int64{Value: instance.LocalStorageTotalGB},
				"cpu_core_count":                  types.Int64{Value: instance.CPUCoreCount},
				"network_mbps":                    types.Int64{Value: instance.NetworkMBPS},
				"external_storage_network_mbps":   types.Int64{Value: instance.ExternalStorageNetworkMBPS},
				"environment":                     types.String{Value: instance.Environment},
				"display_order":                   types.Int64{Value: instance.DisplayOrder},
				"network_speed_description":       types.String{Value: instance.NetworkSpeedDescription},
				"license_cost_on_demand_per_hour": types.String{Value: instance.LicenseCostOnDemandPerHour},
				"free_tier_hours":                 types.Int64{Value: instance.FreeTierHours},
				"instance_family":                 types.String{Value: instance.InstanceFamily},
				"group_default":                   types.Bool{Value: instance.GroupDefault},
				"cost_per_hour":                   types.String{Value: instance.CostPerHour},
				"subscription_cost_hourly":        types.String{Value: instance.SubscriptionCostHourly},
				"subscription_cost_monthly":       types.String{Value: instance.SubscriptionCostMonthly},
				"subscription_cost_yearly":        types.String{Value: instance.SubscriptionCostYearly},
			},
			AttrTypes: instanceAttrsTypes,
		}
	}

	data.All = types.Map{Elems: instancesByName, ElemType: types.ObjectType{AttrTypes: instanceAttrsTypes}}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
