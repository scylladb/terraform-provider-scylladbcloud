package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = allowlistRuleDataSourceType{}
var _ tfsdk.DataSource = allowlistRuleDataSource{}

type allowlistRuleDataSourceType struct{}

func (t allowlistRuleDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Firewall rule data source",

		Attributes: map[string]tfsdk.Attribute{
			"cluster_id": {
				MarkdownDescription: "ID of the cluster",
				Required:            true,
				Type:                types.Int64Type,
			},
			"id": {
				MarkdownDescription: "ID of the rule",
				Computed:            true,
				Optional:            true,
				Type:                types.Int64Type,
			},
			"source_address": {
				MarkdownDescription: "Source address of the rule, where address is an IP address with mask (eg. 83.23.117.37/32)",
				Computed:            true,
				Optional:            true,
				Type:                types.StringType,
			},
		},
	}, nil
}

func (t allowlistRuleDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return allowlistRuleDataSource{provider: provider}, diags
}

type allowlistRuleDataSourceData struct {
	ClusterID     types.Int64  `tfsdk:"cluster_id"`
	ID            types.Int64  `tfsdk:"id"`
	SourceAddress types.String `tfsdk:"source_address"`
}

type allowlistRuleDataSource struct {
	provider provider
}

func (d allowlistRuleDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data allowlistRuleDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.IsNull() && data.SourceAddress.IsNull() {
		resp.Diagnostics.AddError("malformed data", "id or source_address must be specified")
		return
	}

	rules, err := d.provider.client.ListAllowlistRules(data.ClusterID.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cloud provider regions, got error: %s", err))
		return
	}

	foundRule := false
	for _, rule := range rules {
		if (!data.ID.IsNull() && rule.ID == data.ID.Value) || (!data.SourceAddress.IsNull() && rule.SourceAddress == data.SourceAddress.Value) {
			data.ID = types.Int64{Value: rule.ID}
			data.ClusterID = types.Int64{Value: rule.ClusterID}
			data.SourceAddress = types.String{Value: rule.SourceAddress}
			foundRule = true
			break
		}
	}
	if !foundRule {
		resp.Diagnostics.AddError("Not Found", "No rule matching criteria found")
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
