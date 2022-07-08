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
var _ tfsdk.DataSourceType = providerRegionDataSourceType{}
var _ tfsdk.DataSource = providerRegionDataSource{}

type providerRegionDataSourceType struct{}

var regionAttrs = markAttrsAsComputed(map[string]tfsdk.Attribute{
	"id": {
		MarkdownDescription: "ID of the region",
		Type:                types.Int64Type,
	},
	"cloud_provider_id": {
		MarkdownDescription: "ID of the cloud provider",
		Type:                types.Int64Type,
	},
	"name": {
		MarkdownDescription: "Name of the region",
		Type:                types.StringType,
	},
	"full_name": {
		MarkdownDescription: "Full name of the region",
		Type:                types.StringType,
	},
	"external_id": {
		MarkdownDescription: "External ID of the region",
		Type:                types.StringType,
	},
	"multi_region_external_id": {
		MarkdownDescription: "Multi-region external ID of the region",
		Type:                types.StringType,
	},
	"dc_name": {
		MarkdownDescription: "Name of the data center",
		Type:                types.StringType,
	},
	"backup_storage_gb_cost": {
		MarkdownDescription: "Cost of backup storage in GB",
		Type:                types.StringType,
	},
	"traffic_same_region_in_gb_cost": {
		MarkdownDescription: "Cost of traffic in the same region in GB",
		Type:                types.StringType,
	},
	"traffic_same_region_out_gb_cost": {
		MarkdownDescription: "Cost of traffic in the same region out of the region in GB",
		Type:                types.StringType,
	},
	"traffic_cross_region_out_gb_cost": {
		MarkdownDescription: "Cost of traffic in the cross region out of the region in GB",
		Type:                types.StringType,
	},
	"traffic_internet_out_gb_cost": {
		MarkdownDescription: "Cost of traffic out of the region in GB",
		Type:                types.StringType,
	},
	"continent": {
		MarkdownDescription: "Continent of the region",
		Type:                types.StringType,
	},
})

var regionObjectAttrTypes = extractAttrsTypes(regionAttrs)

func (t providerRegionDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Provider regions data source",

		Attributes: map[string]tfsdk.Attribute{
			"provider_id": {
				MarkdownDescription: "ID of the cloud provider",
				Required:            true,
				Type:                types.Int64Type,
			},
			"all": {
				MarkdownDescription: "Map of all regions, where the key is the region code name (eg. us-east-1)",
				Computed:            true,
				Type: types.MapType{
					ElemType: tfsdk.SingleNestedAttributes(regionAttrs).AttributeType(),
				},
			},
		},
	}, nil
}

func (t providerRegionDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return providerRegionDataSource{provider: provider}, diags
}

type providerRegionDataSourceData struct {
	ProviderId types.Int64 `tfsdk:"provider_id"`
	All        types.Map   `tfsdk:"all"`
}

type providerRegionDataSource struct {
	provider provider
}

func (d providerRegionDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data providerRegionDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	regions, err := d.provider.client.ListCloudProviderRegions(data.ProviderId.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cloud provider regions, got error: %s", err))
		return
	}

	regionsByName := make(map[string]attr.Value)
	for _, region := range regions {
		regionsByName[region.Name] = types.Object{
			Attrs: map[string]attr.Value{
				"id":                               types.Int64{Value: region.Id},
				"cloud_provider_id":                types.Int64{Value: region.CloudProviderId},
				"name":                             types.String{Value: region.Name},
				"full_name":                        types.String{Value: region.FullName},
				"external_id":                      types.String{Value: region.ExternalId},
				"multi_region_external_id":         types.String{Value: region.MultiRegionExternalId},
				"dc_name":                          types.String{Value: region.DcName},
				"backup_storage_gb_cost":           types.String{Value: region.BackupStorageGbCost},
				"traffic_same_region_in_gb_cost":   types.String{Value: region.TrafficSameRegionInGbCost},
				"traffic_same_region_out_gb_cost":  types.String{Value: region.TrafficSameRegionOutGbCost},
				"traffic_cross_region_out_gb_cost": types.String{Value: region.TrafficCrossRegionOutGbCost},
				"traffic_internet_out_gb_cost":     types.String{Value: region.TrafficInternetOutGbCost},
				"continent":                        types.String{Value: region.Continent},
			},
			AttrTypes: regionObjectAttrTypes,
		}
	}

	data.All = types.Map{Elems: regionsByName, ElemType: types.ObjectType{AttrTypes: regionObjectAttrTypes}}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
