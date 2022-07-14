package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = providerDataSourceType{}
var _ tfsdk.DataSource = providerDataSource{}

type providerDataSourceType struct{}

func (t providerDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Provider data source",

		Attributes: map[string]tfsdk.Attribute{
			"vendor": {
				MarkdownDescription: "Name of the cloud provider",
				Required:            true,
				Type:                types.StringType,
			},
			"id": {
				MarkdownDescription: "ID of the provider",
				Computed:            true,
				Type:                types.Int64Type,
			},
			"root_account_id": {
				MarkdownDescription: "ID of the root account",
				Computed:            true,
				Type:                types.StringType,
			},
		},
	}, nil
}

func (t providerDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return providerDataSource{provider: provider}, diags
}

type providerDataSourceData struct {
	Vendor        types.String `tfsdk:"vendor"`
	ID            types.Int64  `tfsdk:"id"`
	RootAccountID types.String `tfsdk:"root_account_id"`
}

type providerDataSource struct {
	provider provider
}

func (d providerDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data providerDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	cloudProviders, err := d.provider.client.ListCloudProviders()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cloud providers, got error: %s", err))
		return
	}

	found := false
	for _, cloudProvider := range cloudProviders {
		if cloudProvider.Name == data.Vendor.Value {
			data.ID = types.Int64{Value: cloudProvider.ID}
			data.RootAccountID = types.String{Value: cloudProvider.RootAccountID}
			found = true
		}
	}
	if !found {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find cloud provider with name %s", data.Vendor.Value))
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
