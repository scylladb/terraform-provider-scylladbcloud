package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.ResourceType = allowlistRuleResourceType{}
var _ tfsdk.Resource = allowlistRuleResource{}

type allowlistRuleResourceType struct{}

func (t allowlistRuleResourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Allowlist rule resource",

		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:            true,
				MarkdownDescription: "Rule ID",
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.UseStateForUnknown(),
				},
				Type: types.Int64Type,
			},
			"cluster_id": {
				MarkdownDescription: "Cluster ID",
				Required:            true,
				Type:                types.Int64Type,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.RequiresReplace(),
				},
			},
			"source_address": {
				MarkdownDescription: "The source address of the rule, eg. 150.64.24.10/32",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.RequiresReplace(),
				},
			},
		},
	}, nil
}

func (t allowlistRuleResourceType) NewResource(ctx context.Context, in tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return allowlistRuleResource{
		provider: provider,
	}, diags
}

type allowlistRuleResourceData struct {
	Id            types.Int64  `tfsdk:"id"`
	ClusterId     types.Int64  `tfsdk:"cluster_id"`
	SourceAddress types.String `tfsdk:"source_address"`
}

type allowlistRuleResource struct {
	provider provider
}

func (r allowlistRuleResource) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var data allowlistRuleResourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.provider.client.CreateAllowlistRule(data.ClusterId.Value, data.SourceAddress.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create allowlistRule, got error: %s", err))
		return
	}
	data.Id = types.Int64{Value: rule.Id}

	tflog.Trace(ctx, fmt.Sprintf("created an allowlist rule with ID %d", rule.Id))

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r allowlistRuleResource) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var data allowlistRuleResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.provider.client.GetAllowlistRule(data.ClusterId.Value, data.Id.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read allowlistRule, got error: %s", err))
		return
	}
	data.SourceAddress = types.String{Value: rule.SourceAddress}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r allowlistRuleResource) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	// Intentionally left blank, changing any of the values forces recreation of the resource.
}

func (r allowlistRuleResource) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var data allowlistRuleResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.provider.client.DeleteAllowlistRule(data.ClusterId.Value, data.Id.Value); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete allowlistRule, got error: %s", err))
		return
	}
}
