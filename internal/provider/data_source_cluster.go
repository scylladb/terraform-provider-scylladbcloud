package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/scylladb/terraform-provider-scyllacloud/internal/scyllaCloudSDK"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.DataSourceType = clusterDataSourceType{}
var _ tfsdk.DataSource = clusterDataSource{}

type clusterDataSourceType struct{}

var dcAttrs = markAttrsAsComputed(map[string]tfsdk.Attribute{
	"id": {
		MarkdownDescription: "ID of DC",
		Type:                types.Int64Type,
	},
	"cluster_id": {
		MarkdownDescription: "ID of the cluster",
		Type:                types.Int64Type,
	},
	"cloud_provider_id": {
		MarkdownDescription: "ID of the cloud provider",
		Type:                types.Int64Type,
	},
	"cloud_provider_region_id": {
		MarkdownDescription: "ID of the cloud provider region",
		Type:                types.Int64Type,
	},
	"replication_factor": {
		MarkdownDescription: "Replication factor of the cluster",
		Type:                types.Int64Type,
	},
	"ipv4_cidr": {
		MarkdownDescription: "IPv4 CIDR of the cluster",
		Type:                types.StringType,
	},
	"account_cloud_provider_credential_id": {
		MarkdownDescription: "ID of the account cloud provider credential",
		Type:                types.Int64Type,
	},
	"status": {
		MarkdownDescription: "Status of the cluster",
		Type:                types.StringType,
	},
	"name": {
		MarkdownDescription: "Name of the cluster",
		Type:                types.StringType,
	},
	"management_network": {
		MarkdownDescription: "Management network of the cluster",
		Type:                types.StringType,
	},
	"instance_type_id": {
		MarkdownDescription: "ID of the instance type",
		Type:                types.Int64Type,
	},
	"client_connection": {
		MarkdownDescription: "Client connection of the cluster",
		Type: types.ListType{
			ElemType: types.StringType,
		},
	},
})

var dcAttrTypes = extractAttrsTypes(dcAttrs)

var freeTierAttrs = markAttrsAsComputed(map[string]tfsdk.Attribute{
	"expiration_date": {
		MarkdownDescription: "Expiration date of the free tier",
		Type:                types.StringType,
	},
	"expiration_seconds": {
		MarkdownDescription: "Expiration seconds of the free tier",
		Type:                types.Int64Type,
	},
	"creation_time": {
		MarkdownDescription: "Creation time of the free tier",
		Type:                types.StringType,
	},
})

var freeTierAttrsTypes = extractAttrsTypes(freeTierAttrs)

func (t clusterDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	attrs := markAttrsAsComputed(map[string]tfsdk.Attribute{
		"id": {
			MarkdownDescription: "Cluster id",
			Optional:            true,
			Type:                types.Int64Type,
		},
		"name": {
			MarkdownDescription: "Cluster name",
			Optional:            true,
			Type:                types.StringType,
		},
		"cluster_name_on_config_file": {
			MarkdownDescription: "Cluster name on config file",
			Type:                types.StringType,
		},
		"status": {
			MarkdownDescription: "Cluster status",
			Type:                types.StringType,
		},
		"cloud_provider_id": {
			MarkdownDescription: "Cloud provider id",
			Type:                types.Int64Type,
		},
		"replication_factor": {
			MarkdownDescription: "Cluster replication factor",
			Type:                types.Int64Type,
		},
		"broadcast_type": {
			MarkdownDescription: "Cluster broadcast type",
			Type:                types.StringType,
		},
		"scylla_version_id": {
			MarkdownDescription: "Scylla version id",
			Type:                types.Int64Type,
		},
		"scylla_version": {
			MarkdownDescription: "Scylla version",
			Type:                types.StringType,
		},
		"dc": {
			MarkdownDescription: "Datacenters",
			Attributes:          tfsdk.ListNestedAttributes(dcAttrs),
		},
		"grafana_url": {
			MarkdownDescription: "Grafana url",
			Type:                types.StringType,
		},
		"grafana_root_url": {
			MarkdownDescription: "Grafana root url",
			Type:                types.StringType,
		},
		"backoffice_grafana_url": {
			MarkdownDescription: "Backoffice grafana url",
			Type:                types.StringType,
		},
		"backoffice_prometheus_url": {
			MarkdownDescription: "Backoffice prometheus url",
			Type:                types.StringType,
		},
		"backoffice_alert_manager_url": {
			MarkdownDescription: "Backoffice alert manager url",
			Type:                types.StringType,
		},
		"free_tier": {
			MarkdownDescription: "Free tier",
			Attributes:          tfsdk.SingleNestedAttributes(freeTierAttrs),
		},
		"encryption_mode": {
			MarkdownDescription: "Encryption mode",
			Type:                types.StringType,
		},
		"user_api_interface": {
			MarkdownDescription: "User api interface",
			Type:                types.StringType,
		},
		"pricing_model": {
			MarkdownDescription: "Pricing model",
			Type:                types.Int64Type,
		},
		"max_allowed_cidr_range": {
			MarkdownDescription: "Max allowed cidr range",
			Type:                types.Int64Type,
		},
		"created_at": {
			MarkdownDescription: "Created at",
			Type:                types.StringType,
		},
		"dns": {
			MarkdownDescription: "Dns",
			Type:                types.BoolType,
		},
		"prom_proxy_enabled": {
			MarkdownDescription: "Prom proxy enabled",
			Type:                types.BoolType,
		},
	})

	return tfsdk.Schema{
		MarkdownDescription: "Clusters data source",
		Attributes:          attrs,
	}, nil
}

func (t clusterDataSourceType) NewDataSource(ctx context.Context, in tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return clusterDataSource{provider: provider}, diags
}

type clusterDataSourceData struct {
	Id                        types.Int64  `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	ClusterNameOnConfigFile   types.String `tfsdk:"cluster_name_on_config_file"`
	Status                    types.String `tfsdk:"status"`
	CloudProviderId           types.Int64  `tfsdk:"cloud_provider_id"`
	ReplicationFactor         types.Int64  `tfsdk:"replication_factor"`
	BroadcastType             types.String `tfsdk:"broadcast_type"`
	ScyllaVersionId           types.Int64  `tfsdk:"scylla_version_id"`
	ScyllaVersion             types.String `tfsdk:"scylla_version"`
	Dc                        types.List   `tfsdk:"dc"`
	GrafanaUrl                types.String `tfsdk:"grafana_url"`
	GrafanaRootUrl            types.String `tfsdk:"grafana_root_url"`
	BackofficeGrafanaUrl      types.String `tfsdk:"backoffice_grafana_url"`
	BackofficePrometheusUrl   types.String `tfsdk:"backoffice_prometheus_url"`
	BackofficeAlertManagerUrl types.String `tfsdk:"backoffice_alert_manager_url"`
	FreeTier                  types.Object `tfsdk:"free_tier"`
	EncryptionMode            types.String `tfsdk:"encryption_mode"`
	UserApiInterface          types.String `tfsdk:"user_api_interface"`
	PricingModel              types.Int64  `tfsdk:"pricing_model"`
	MaxAllowedCidrRange       types.Int64  `tfsdk:"max_allowed_cidr_range"`
	CreatedAt                 types.String `tfsdk:"created_at"`
	Dns                       types.Bool   `tfsdk:"dns"`
	PromProxyEnabled          types.Bool   `tfsdk:"prom_proxy_enabled"`
}

type clusterDataSource struct {
	provider provider
}

func (d clusterDataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var data clusterDataSourceData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError("failed to match cluster", "at least one of {id, name} must be specified")
		return
	}

	clusters, err := d.provider.client.ListClusters()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list clusters, got error: %s", err))
		return
	}

	matchedClusterIdx := -1
	for i, cluster := range clusters {
		if !data.Id.IsNull() {
			if cluster.Id != data.Id.Value {
				continue
			}
		}
		if !data.Name.IsNull() {
			if cluster.Name != data.Name.Value {
				continue
			}
		}
		matchedClusterIdx = i
		break
	}

	if matchedClusterIdx == -1 {
		resp.Diagnostics.AddError("failed to match cluster", "no cluster found")
		return
	}

	writeClusterDataToTFStruct(&clusters[matchedClusterIdx], &data)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func writeClusterDataToTFStruct(cluster *scyllaCloudSDK.Cluster, data *clusterDataSourceData) {
	data.Id = types.Int64{Value: cluster.Id}
	data.Name = types.String{Value: cluster.Name}
	data.ClusterNameOnConfigFile = types.String{Value: cluster.ClusterNameOnConfigFile}
	data.Status = types.String{Value: cluster.Status}
	data.CloudProviderId = types.Int64{Value: cluster.CloudProviderId}
	data.ReplicationFactor = types.Int64{Value: cluster.ReplicationFactor}
	data.BroadcastType = types.String{Value: cluster.BroadcastType}
	data.ScyllaVersionId = types.Int64{Value: cluster.ScyllaVersionId}
	data.ScyllaVersion = types.String{Value: cluster.ScyllaVersion}

	data.GrafanaUrl = types.String{Value: cluster.GrafanaUrl}
	data.GrafanaRootUrl = types.String{Value: cluster.GrafanaRootUrl}
	data.BackofficeGrafanaUrl = types.String{Value: cluster.BackofficeGrafanaUrl}
	data.BackofficePrometheusUrl = types.String{Value: cluster.BackofficePrometheusUrl}
	data.BackofficeAlertManagerUrl = types.String{Value: cluster.BackofficeAlertManagerUrl}
	data.EncryptionMode = types.String{Value: cluster.EncryptionMode}
	data.UserApiInterface = types.String{Value: cluster.UserApiInterface}
	data.PricingModel = types.Int64{Value: cluster.PricingModel}
	data.MaxAllowedCidrRange = types.Int64{Value: cluster.MaxAllowedCidrRange}
	data.CreatedAt = types.String{Value: cluster.CreatedAt}
	data.Dns = types.Bool{Value: cluster.Dns}
	data.PromProxyEnabled = types.Bool{Value: cluster.PromProxyEnabled}

	dcs := make([]attr.Value, len(cluster.Dc))
	for i, dc := range cluster.Dc {
		clientConnections := make([]attr.Value, len(dc.ClientConnection))
		for j, clientConnection := range dc.ClientConnection {
			clientConnections[j] = types.String{Value: clientConnection}
		}

		dcs[i] = types.Object{
			AttrTypes: dcAttrTypes,
			Attrs: map[string]attr.Value{
				"id":                                   types.Int64{Value: dc.Id},
				"cluster_id":                           types.Int64{Value: dc.ClusterId},
				"cloud_provider_id":                    types.Int64{Value: dc.CloudProviderId},
				"cloud_provider_region_id":             types.Int64{Value: dc.CloudProviderRegionId},
				"replication_factor":                   types.Int64{Value: dc.ReplicationFactor},
				"ipv4_cidr":                            types.String{Value: dc.Ipv4Cidr},
				"account_cloud_provider_credential_id": types.Int64{Value: dc.AccountCloudProviderCredentialId},
				"status":                               types.String{Value: dc.Status},
				"name":                                 types.String{Value: dc.Name},
				"management_network":                   types.String{Value: dc.ManagementNetwork},
				"instance_type_id":                     types.Int64{Value: dc.InstanceTypeId},
				"client_connection": types.List{
					ElemType: types.StringType,
					Elems:    clientConnections,
				},
			},
		}
	}

	data.Dc = types.List{Elems: dcs, ElemType: types.ObjectType{AttrTypes: dcAttrTypes}}

	data.FreeTier = types.Object{
		AttrTypes: freeTierAttrsTypes,
		Attrs: map[string]attr.Value{
			"expiration_date":    types.String{Value: cluster.FreeTier.ExpirationDate},
			"expiration_seconds": types.Int64{Value: cluster.FreeTier.ExpirationSeconds},
			"creation_time":      types.String{Value: cluster.FreeTier.CreationTime},
		},
	}
}
