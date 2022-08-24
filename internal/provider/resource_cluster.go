package provider

import (
	"context"
	"fmt"
	"github.com/scylladb/terraform-provider-scyllacloud/internal/scyllaCloudSDK"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.ResourceType = clusterResourceType{}
var _ tfsdk.Resource = clusterResource{}
var _ tfsdk.ResourceWithImportState = clusterResource{}

type clusterResourceType struct{}

func (t clusterResourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	//	region   = scylla_cloud_provider_instance.available.region
	//	instance = scylla_cloud_provider_instance.available.all[“i3.2xlarge”].id
	//
	//	count               = 3
	//	cidr                = 172.32.0.0/16
	//	vpc_peering_enabled = true

	attrs := map[string]tfsdk.Attribute{
		"id": {
			MarkdownDescription: "Cluster id",
			Computed:            true,
			Type:                types.Int64Type,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				tfsdk.UseStateForUnknown(),
			},
		},
		"name": {
			MarkdownDescription: "Cluster name",
			Required:            true,
			Type:                types.StringType,
		},
		"name_on_config_file": {
			MarkdownDescription: "Cluster name on config file",
			Computed:            true,
			Type:                types.StringType,
		},
		"status": {
			MarkdownDescription: "Cluster status",
			Computed:            true,
			Type:                types.StringType,
		},
		"provider_id": {
			MarkdownDescription: "Cloud provider id",
			Required:            true,
			Type:                types.Int64Type,
		},
		"replication_factor": {
			MarkdownDescription: "Cluster replication factor",
			Required:            true,
			Type:                types.Int64Type,
		},
		"broadcast_type": { // TODO vpc_peering_enabled
			MarkdownDescription: "Cluster broadcast type",
			Required:            true,
			Type:                types.StringType,
		},
		"scylla_version_id": {
			MarkdownDescription: "Scylla version id",
			Computed:            true,
			Type:                types.Int64Type,
		},
		"scylla_version": {
			MarkdownDescription: "Scylla version",
			Computed:            true,
			Type:                types.StringType,
		},
		"dc": {
			MarkdownDescription: "Data centers",
			Computed:            true,
			Type:                types.ListType{ElemType: types.ObjectType{AttrTypes: dcAttrTypes}},
		},
		"grafana_url": {
			MarkdownDescription: "Grafana url",
			Computed:            true,
			Type:                types.StringType,
		},
		"grafana_root_url": {
			MarkdownDescription: "Grafana root url",
			Computed:            true,
			Type:                types.StringType,
		},
		"free_tier": {
			MarkdownDescription: "Free tier",
			Computed:            true,
			Type:                types.ObjectType{AttrTypes: freeTierAttrsTypes},
		},
		"encryption_mode": {
			MarkdownDescription: "Encryption mode",
			Computed:            true,
			Type:                types.StringType,
		},
		"user_api_interface": {
			MarkdownDescription: "User api interface",
			Required:            true,
			Type:                types.StringType,
		},
		"pricing_model": {
			MarkdownDescription: "Pricing model",
			Computed:            true,
			Type:                types.Int64Type,
		},
		"max_allowed_cidr_range": {
			MarkdownDescription: "Max allowed cidr range",
			Computed:            true,
			Type:                types.Int64Type,
		},
		"created_at": {
			MarkdownDescription: "Created at",
			Computed:            true,
			Type:                types.StringType,
		},
		"dns": {
			MarkdownDescription: "Dns",
			Computed:            true,
			Type:                types.BoolType,
		},
		"prom_proxy_enabled": {
			MarkdownDescription: "Prom proxy enabled",
			Computed:            true,
			Type:                types.BoolType,
		},
		///
		"instance_type_id": {
			MarkdownDescription: "ID of the instance type",
			Required:            true,
			Type:                types.Int64Type,
		},
		"provider_region_id": {
			MarkdownDescription: "ID of the cloud provider region",
			Required:            true,
			Type:                types.Int64Type,
		},
		"cidr": {
			MarkdownDescription: "IPv4 CIDR of the cluster",
			Optional:            true,
			Type:                types.StringType,
		},
	}

	return tfsdk.Schema{
		MarkdownDescription: "Scylla cluster resource",
		Attributes:          attrs,
	}, nil
}

func (t clusterResourceType) NewResource(ctx context.Context, in tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return clusterResource{provider: provider}, diags
}

type clusterResourceData = clusterDataSourceData

type clusterResource struct {
	provider provider
}

func (r clusterResource) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var data clusterResourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement
	panic("create is not yet implemented, it should not be invoked during tests")

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// example, err := d.provider.client.CreateExample(...)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create example, got error: %s", err))
	//     return
	// }

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	// data.Id = types.String{Value: "example-id"}

	// write logs using the tflog package
	// see https://pkg.go.dev/github.com/hashicorp/terraform-plugin-log/tflog
	// for more information
	// tflog.Trace(ctx, "created a resource")

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r clusterResource) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var data clusterResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var cluster scyllaCloudSDK.Cluster
	var err error

	if !data.Id.IsNull() && !data.Id.IsUnknown() {
		cluster, err = r.provider.client.GetCluster(data.Id.Value)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster data, got error: %s", err))
			return
		}
	} else if !data.Name.IsNull() && !data.Name.IsUnknown() {
		clusters, err := r.provider.client.ListClusters()
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list clusters, got error: %s", err))
			return
		}
		for _, c := range clusters {
			if c.Name == data.Name.Value {
				cluster = c
				break
			}
		}
		if cluster.Name != data.Name.Value {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find cluster with name %s", data.Name.Value))
			return
		}
	} else {
		resp.Diagnostics.AddError("Read Error", "Either id or name must be provided and known")
		return
	}

	writeClusterDataToTFStruct(&cluster, &data)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r clusterResource) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	var data clusterResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement
	panic("update is not yet implemented, it should not be invoked during tests")

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// example, err := d.provider.client.UpdateExample(...)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update example, got error: %s", err))
	//     return
	// }

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r clusterResource) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var data clusterResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: delete the cluster
	panic("delete is not yet implemented, it should not be invoked during tests")

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// example, err := d.provider.client.DeleteExample(...)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete example, got error: %s", err))
	//     return
	// }
}

func (r clusterResource) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	tfsdk.ResourceImportStatePassthroughID(ctx, tftypes.NewAttributePath().WithAttributeName("name"), req, resp)
}
