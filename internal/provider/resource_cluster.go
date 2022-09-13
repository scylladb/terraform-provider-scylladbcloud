package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/scylladb/terraform-provider-scylla/internal/scylla/model"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const PollInterval = 1 * time.Second

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
			MarkdownDescription: "DNS",
			Computed:            true,
			Type:                types.BoolType,
		},
		"prom_proxy_enabled": {
			MarkdownDescription: "Prom proxy enabled",
			Computed:            true,
			Type:                types.BoolType,
		},
		///
		"nodes": {
			MarkdownDescription: "Size of the cluster",
			Required:            true,
			Type:                types.Int64Type,
		},
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

type clusterResourceData struct {
	ID                  types.Int64  `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	NameOnConfigFile    types.String `tfsdk:"name_on_config_file"`
	Status              types.String `tfsdk:"status"`
	ProviderID          types.Int64  `tfsdk:"provider_id"`
	ReplicationFactor   types.Int64  `tfsdk:"replication_factor"`
	BroadcastType       types.String `tfsdk:"broadcast_type"`
	ScyllaVersionID     types.Int64  `tfsdk:"scylla_version_id"`
	ScyllaVersion       types.String `tfsdk:"scylla_version"`
	DC                  types.List   `tfsdk:"dc"`
	GrafanaURL          types.String `tfsdk:"grafana_url"`
	GrafanaRootURL      types.String `tfsdk:"grafana_root_url"`
	FreeTier            types.Object `tfsdk:"free_tier"`
	EncryptionMode      types.String `tfsdk:"encryption_mode"`
	UserApiInterface    types.String `tfsdk:"user_api_interface"`
	PricingModel        types.Int64  `tfsdk:"pricing_model"`
	MaxAllowedCidrRange types.Int64  `tfsdk:"max_allowed_cidr_range"`
	CreatedAt           types.String `tfsdk:"created_at"`
	DNS                 types.Bool   `tfsdk:"dns"`
	PromProxyEnabled    types.Bool   `tfsdk:"prom_proxy_enabled"`

	Nodes                  types.Int64  `tfsdk:"nodes"`
	ProviderRegionID       types.Int64  `tfsdk:"provider_region_id"`
	ProviderInstanceTypeID types.Int64  `tfsdk:"instance_type_id"`
	CIDR                   types.String `tfsdk:"cidr"`
}

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

	clusterData := map[string]interface{}{
		"name":                      data.Name.Value,
		"nodes":                     data.Nodes.Value,
		"cloudProvider":             data.ProviderID.Value,
		"cloudProviderRegion":       data.ProviderRegionID.Value,
		"cloudProviderInstanceType": data.ProviderInstanceTypeID.Value,
		"replicationFactor":         data.ReplicationFactor.Value,
	}
	if !data.CIDR.IsNull() {
		clusterData["cidrBlock"] = data.CIDR.Value
	}
	if !data.BroadcastType.IsNull() {
		clusterData["broadcastType"] = data.BroadcastType.Value
	}
	if !data.DNS.IsNull() {
		clusterData["enableDnsAssociation"] = data.DNS.Value
	}
	if !data.UserApiInterface.IsNull() {
		clusterData["userApiInterface"] = data.UserApiInterface.Value
	}

	createRequest, err := r.provider.client.CreateCluster(clusterData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to send create cluster request, got error: %s", err))
		return
	}

	if createRequest.Status != "QUEUED" {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create cluster, got status: %s", createRequest.Status))
		return
	}

	for {
		time.Sleep(PollInterval)
		request, err := r.provider.client.GetClusterRequest(createRequest.ID)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster delete request status: %s", err))
			return
		}
		if request.Status == "COMPLETED" {
			break
		}
		if request.Status == "QUEUED" || request.Status == "IN_PROGRESS" {
			continue
		}

		// TODO are there any other possible statuses?

		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster create request status: %s", request.Status))
		return
	}

	tflog.Trace(ctx, "created a cluster")

	cluster, err := r.provider.client.GetCluster(createRequest.ClusterID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster data, got error: %s", err))
		return
	}

	writeClusterResourceDataToTFStruct(&cluster, &data)

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

	var cluster model.Cluster
	var err error

	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		cluster, err = r.provider.client.GetCluster(data.ID.Value)
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

	writeClusterResourceDataToTFStruct(&cluster, &data)

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

	// TODO: think about what can be updated and what should force recreation
	panic("update is not yet implemented, it should not be invoked during tests")

	// diags = resp.State.Set(ctx, &data)
	// resp.Diagnostics.Append(diags...)
}

func (r clusterResource) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var data clusterResourceData
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	deleteRequest, err := r.provider.client.DeleteCluster(data.ID.Value, data.Name.Value)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to send delete cluster request, got error: %s", err))
		return
	}

	if deleteRequest.Status != "QUEUED" {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete cluster, got status: %s", deleteRequest.Status))
		return
	}

	for {
		time.Sleep(PollInterval)
		request, err := r.provider.client.GetClusterRequest(deleteRequest.ID)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster delete request status: %s", err))
			return
		}
		if request.Status == "COMPLETED" {
			break
		}
		if request.Status == "QUEUED" || request.Status == "IN_PROGRESS" {
			continue
		}

		// TODO are there any other possible statuses?

		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster delete request status: %s", request.Status))
		return
	}
}

func (r clusterResource) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	tfsdk.ResourceImportStatePassthroughID(ctx, tftypes.NewAttributePath().WithAttributeName("name"), req, resp)
}

func writeClusterResourceDataToTFStruct(cluster *model.Cluster, data *clusterResourceData) {
	data.ID = types.Int64{Value: cluster.ID}
	data.Name = types.String{Value: cluster.Name}
	data.NameOnConfigFile = types.String{Value: cluster.ClusterNameOnConfigFile}
	data.Status = types.String{Value: cluster.Status}
	data.ProviderID = types.Int64{Value: cluster.CloudProviderID}
	data.ReplicationFactor = types.Int64{Value: cluster.ReplicationFactor}
	data.BroadcastType = types.String{Value: cluster.BroadcastType}
	data.ScyllaVersionID = types.Int64{Value: cluster.ScyllaVersionID}
	data.ScyllaVersion = types.String{Value: cluster.ScyllaVersion}
	data.GrafanaURL = types.String{Value: cluster.GrafanaURL}
	data.GrafanaRootURL = types.String{Value: cluster.GrafanaRootURL}
	data.EncryptionMode = types.String{Value: cluster.EncryptionMode}
	data.UserApiInterface = types.String{Value: cluster.UserApiInterface}
	data.PricingModel = types.Int64{Value: cluster.PricingModel}
	data.MaxAllowedCidrRange = types.Int64{Value: cluster.MaxAllowedCIDRRange}
	data.CreatedAt = types.String{Value: cluster.CreatedAt}
	data.DNS = types.Bool{Value: cluster.DNS}
	data.PromProxyEnabled = types.Bool{Value: cluster.PromProxyEnabled}
	data.DC = parseClusterDCData(cluster.Dc)
	data.FreeTier = parseClusterFreeTierData(&cluster.FreeTier)
}
