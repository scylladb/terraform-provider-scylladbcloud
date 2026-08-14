package cluster

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	clusterRetryTimeout  = 40 * time.Minute
	clusterDeleteTimeout = 90 * time.Minute
	clusterPollInterval  = 10 * time.Second
)

func validateMinNodesDiag(v interface{}, _ cty.Path) diag.Diagnostics {
	value := v.(int)
	if value < 3 {
		return diag.Errorf("min_nodes must be at least 3, got %d", value)
	}
	if value%3 != 0 {
		return diag.Errorf("min_nodes must be divisible by 3, got %d", value)
	}
	return nil
}

func validateScalingTargetUtilizationDiag(v interface{}, _ cty.Path) diag.Diagnostics {
	value := v.(float64)
	if value <= 0.0 || value > 0.9 {
		return diag.Errorf("target_utilization must be greater than 0.0 and less than or equal to 0.9, got %v", value)
	}
	return nil
}

func validateBackupRetentionDaysDiag(v interface{}, _ cty.Path) diag.Diagnostics {
	value := v.(int)
	if value < 0 || value > 60 {
		return diag.Errorf("backup_retention_days must be between 0 and 60, got %d", value)
	}
	return nil
}

func castToNestedBlock(raw interface{}) (map[string]interface{}, bool) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 || items[0] == nil {
		return nil, false
	}

	block, ok := items[0].(map[string]interface{})
	return block, ok
}

func isNonEmptyList(raw interface{}) bool {
	items, ok := raw.([]interface{})
	return ok && len(items) > 0
}

func castToStringList(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			continue
		}
		out = append(out, s)
	}

	return out
}

func expandScaling(raw interface{}, region string, instances []model.CloudProviderInstance, cloudProvider *scylla.CloudProvider) (*model.Scaling, error) {
	block, ok := castToNestedBlock(raw)
	if !ok {
		return nil, nil
	}

	scaling := &model.Scaling{
		InstanceFamilies: castToStringList(block["instance_families"]),
	}

	for _, family := range scaling.InstanceFamilies {
		if i := cloudProvider.InstanceByFamilyNameFromInstances(family, instances); i == nil {
			return nil, fmt.Errorf("unsupported scaling instance_family %q in region %s", family, region)
		}
	}

	if instanceTypes := castToStringList(block["instance_types"]); len(instanceTypes) > 0 {
		scaling.InstanceTypeIDs = make([]int64, 0, len(instanceTypes))
		for _, instanceType := range instanceTypes {
			instance := cloudProvider.InstanceByNameFromInstances(instanceType, instances)
			if instance == nil {
				return nil, fmt.Errorf("unsupported scaling instance_type %q in region %s", instanceType, region)
			}
			scaling.InstanceTypeIDs = append(scaling.InstanceTypeIDs, instance.ID)
		}
	}

	if storagePolicy, ok := castToNestedBlock(block["storage_policy"]); ok {
		if scaling.Policies == nil {
			scaling.Policies = &model.ScalingPolicies{}
		}
		scaling.Policies.Storage = &model.ScalingStoragePolicy{
			Min:               int64(storagePolicy["min_gb"].(int)),
			TargetUtilization: storagePolicy["target_utilization"].(float64),
		}
	}

	if vcpuPolicy, ok := castToNestedBlock(block["vcpu_policy"]); ok {
		if scaling.Policies == nil {
			scaling.Policies = &model.ScalingPolicies{}
		}
		scaling.Policies.VCPU = &model.ScalingVCPUPolicy{
			Min: int64(vcpuPolicy["min"].(int)),
		}
	}

	if !scaling.Enabled() {
		return nil, nil
	}
	scaling.Mode = model.ScalingXCloud

	return scaling, nil
}

func flattenScaling(raw *model.Scaling, instances []model.CloudProviderInstance, cloudProvider *scylla.CloudProvider) ([]map[string]interface{}, error) {
	if raw == nil || !raw.Enabled() {
		return []map[string]interface{}{}, nil
	}

	block := map[string]interface{}{}

	if len(raw.InstanceFamilies) > 0 {
		block["instance_families"] = raw.InstanceFamilies
	}

	if len(raw.InstanceTypeIDs) > 0 {
		instanceTypes := make([]string, 0, len(raw.InstanceTypeIDs))
		for _, id := range raw.InstanceTypeIDs {
			instance := cloudProvider.InstanceByIDFromInstances(id, instances)
			if instance == nil {
				return nil, fmt.Errorf("unexpected scaling instance type ID %d", id)
			}
			instanceTypes = append(instanceTypes, instance.ExternalID)
		}
		block["instance_types"] = instanceTypes
	}

	if raw.Policies != nil {
		if raw.Policies.Storage != nil {
			block["storage_policy"] = []map[string]interface{}{{
				"min_gb":             int(raw.Policies.Storage.Min),
				"target_utilization": raw.Policies.Storage.TargetUtilization,
			}}
		}
		if raw.Policies.VCPU != nil {
			block["vcpu_policy"] = []map[string]interface{}{{
				"min": int(raw.Policies.VCPU.Min),
			}}
		}
	}

	return []map[string]interface{}{block}, nil
}

// encryptionKeyProviders maps the cloud provider name to the pair of
// encryption-at-rest key providers the API accepts.
var encryptionKeyProviders = map[string]struct{ scyllaManaged, customerManaged string }{
	"aws": {model.EncryptionProviderScyllaAWS, model.EncryptionProviderAWS},
	"gcp": {model.EncryptionProviderScyllaGCP, model.EncryptionProviderGCP},
}

func isCustomerManagedKeyProvider(provider string) bool {
	return provider == model.EncryptionProviderAWS || provider == model.EncryptionProviderGCP
}

// expandEncryptionAtRest derives the API request object from the
// encryption_at_rest block. The key provider is derived from cloud so that
// users never have to restate AWS/GCP. A nil result means the field is left out
// of the request entirely, which is how encryption at rest is opted out of.
//
// Omitting the block selects a ScyllaDB-managed key, matching the desired default.
// Opting out takes an explicit "enabled = false".
func expandEncryptionAtRest(cloud string, raw interface{}) (*model.EncryptionAtRest, error) {
	block, configured := castToNestedBlock(raw)

	if configured {
		if enabled, _ := block["enabled"].(bool); !enabled {
			return nil, nil
		}
	}

	providers, ok := encryptionKeyProviders[strings.ToLower(cloud)]
	if !ok {
		if !configured {
			// The default must never fail a create for a cloud the user did
			// not ask to encrypt on.
			return nil, nil
		}
		return nil, fmt.Errorf("encryption at rest is not supported for cloud %q", cloud)
	}

	// key_id is Optional+Computed, but reading it off the diff is safe here:
	// create only ever runs against a null prior state - Terraform re-plans a
	// replacement with a null prior - so this is the configured key or nothing,
	// never a read of an outdated one.
	if keyID, _ := block["key_id"].(string); keyID != "" {
		return &model.EncryptionAtRest{Provider: providers.customerManaged, KeyID: keyID}, nil
	}

	return &model.EncryptionAtRest{Provider: providers.scyllaManaged}, nil
}

// createClusterWithEncryptionFallback creates the cluster, retrying once
// without encryption at rest when the account is not entitled to it and the
// user never asked for it.
//
// Encryption at rest is on by default, so an account without the entitlement
// would otherwise fail every "terraform apply" over a setting it did not
// choose. The retry is deliberately narrow: only error 040723, which the API
// rejects on the cluster POST before anything is provisioned. Retrying on any
// other error risks creating a second cluster, because CreateCluster also
// fetches the cluster request it just enqueued.
//
// An explicitly configured block always fails hard - a user who asked for
// encryption must not silently get an unencrypted cluster.
func createClusterWithEncryptionFallback(
	ctx context.Context,
	c *scylla.Client,
	req *model.ClusterCreateRequest,
	encryptionConfigured bool,
) (*model.ClusterRequest, diag.Diagnostics, error) {
	cr, err := c.CreateCluster(ctx, req)
	if err == nil {
		return cr, nil, nil
	}

	if encryptionConfigured || req.EncryptionAtRest == nil || !scylla.IsEncryptionAtRestUnavailableErr(err) {
		return nil, nil, err
	}

	tflog.Debug(ctx, "Account is not entitled to encryption at rest; retrying without it", map[string]interface{}{
		"cluster_name": req.ClusterName,
	})

	req.EncryptionAtRest = nil

	cr, err = c.CreateCluster(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	return cr, diag.Diagnostics{{
		Severity: diag.Warning,
		Summary:  fmt.Sprintf("Cluster %q was created without encryption at rest", req.ClusterName),
		Detail: "This account is not entitled to encryption at rest, which new clusters use by " +
			"default. The cluster was created unencrypted. Contact ScyllaDB Cloud support to " +
			`enable it, or set "enabled = false" in the "encryption_at_rest" block to silence ` +
			"this warning.",
	}}, nil
}

// flattenEncryptionAtRest always returns exactly one block, never an empty
// list. An unencrypted cluster reads back as "encryption_at_rest { enabled = false }".
// Returning an empty list would leave diffing against nothing forever.
func flattenEncryptionAtRest(raw *model.EncryptionAtRest) []map[string]interface{} {
	if raw == nil || raw.Provider == "" {
		return []map[string]interface{}{{
			"enabled":  false,
			"key_id":   "",
			"provider": "",
		}}
	}

	return []map[string]interface{}{{
		"enabled":  true,
		"key_id":   raw.KeyID,
		"provider": raw.Provider,
	}}
}

func isScalingEqual(lhs, rhs *model.Scaling) bool {
	if lhs == nil || !lhs.Enabled() {
		lhs = nil
	}
	if rhs == nil || !rhs.Enabled() {
		rhs = nil
	}

	if lhs == nil || rhs == nil {
		return lhs == rhs
	}

	if lhs.Mode != rhs.Mode {
		return false
	}
	if !slices.Equal(lhs.InstanceFamilies, rhs.InstanceFamilies) {
		return false
	}
	if !slices.Equal(lhs.InstanceTypeIDs, rhs.InstanceTypeIDs) {
		return false
	}

	return isScalingPoliciesEqual(lhs.Policies, rhs.Policies)
}

func isScalingPoliciesEqual(lhs, rhs *model.ScalingPolicies) bool {
	if lhs == nil || rhs == nil {
		return lhs == rhs
	}
	if !isScalingStoragePolicyEqual(lhs.Storage, rhs.Storage) {
		return false
	}
	if !isScalingVCPUPolicyEqual(lhs.VCPU, rhs.VCPU) {
		return false
	}
	return true
}

func isScalingStoragePolicyEqual(lhs, rhs *model.ScalingStoragePolicy) bool {
	if lhs == nil || rhs == nil {
		return lhs == rhs
	}
	return lhs.Min == rhs.Min && lhs.TargetUtilization == rhs.TargetUtilization
}

func isScalingVCPUPolicyEqual(lhs, rhs *model.ScalingVCPUPolicy) bool {
	if lhs == nil || rhs == nil {
		return lhs == rhs
	}
	return lhs.Min == rhs.Min
}

func hasScaling(cluster *model.Cluster) bool {
	if cluster == nil {
		return false
	}

	if cluster.Datacenter != nil && cluster.Datacenter.Scaling != nil && cluster.Datacenter.Scaling.Enabled() {
		return true
	}

	if len(cluster.Datacenters) == 1 {
		for _, dc := range cluster.Datacenters {
			if dc.Scaling != nil && dc.Scaling.Enabled() {
				return true
			}
		}
	}

	return false
}

func validateScaling(hasMinNodes, hasNodeType bool, scaling map[string]interface{}) error {
	if scaling != nil {
		hasInstanceFamilies := isNonEmptyList(scaling["instance_families"])
		hasInstanceTypes := isNonEmptyList(scaling["instance_types"])

		if hasInstanceFamilies == hasInstanceTypes {
			return fmt.Errorf(`exactly one of "instance_families" or "instance_types" must be configured in the "scaling" block`)
		}

		return nil
	}

	if !hasMinNodes && !hasNodeType {
		return fmt.Errorf(`either configure the "scaling" block or set both "min_nodes" and "node_type"`)
	}
	if !hasMinNodes {
		return fmt.Errorf(`"min_nodes" is required when the "scaling" block is not configured`)
	}
	if !hasNodeType {
		return fmt.Errorf(`"node_type" is required when the "scaling" block is not configured`)
	}

	return nil
}

// encryptionKeyIDRegexp mirrors the pattern the backend router enforces on
// customer-managed key IDs.
var encryptionKeyIDRegexp = regexp.MustCompile(`^key-[a-zA-Z0-9]+$`)

// validateEncryptionAtRest checks the configured encryption_at_rest block.
//
// keyID must come from the raw configuration rather than from the diff because
// the diff falls back to the key ID reported by the API.
func validateEncryptionAtRest(cloud string, enabled bool, keyID string) error {
	if !enabled {
		if keyID != "" {
			return fmt.Errorf(`"key_id" cannot be set when "enabled" is false in the "encryption_at_rest" block`)
		}
		return nil
	}

	if keyID != "" && !encryptionKeyIDRegexp.MatchString(keyID) {
		return fmt.Errorf(`invalid "key_id" %q in the "encryption_at_rest" block, expected a portal key ID such as "key-deadbeef"`, keyID)
	}

	if _, ok := encryptionKeyProviders[strings.ToLower(cloud)]; !ok {
		return fmt.Errorf("encryption at rest is not supported for cloud %q", cloud)
	}

	return nil
}

// validateEncryptionKeyIDNotRemoved reports dropping a customer-managed
// "key_id" from the configuration as an error.
//
// "key_id" is Optional+Computed so that the key ID the API returns can be read
// back into state, and the SDK suppresses the diff when an Optional+Computed
// value disappears from the configuration. Encryption at rest is also
// create-time only, so the removal can neither be applied in place nor planned
// as a replacement. Reporting it avoids confusion compared to silently ignoring it.
func validateEncryptionKeyIDNotRemoved(d *schema.ResourceDiff) error {
	if d.Id() == "" {
		return nil
	}

	configuredKeyID, blockConfigured := configuredEncryptionAtRest(d.GetRawConfig())
	if !blockConfigured || configuredKeyID != "" {
		// Dropping the whole block is the documented Optional+Computed no-op,
		// the same as for every other read back attribute on this resource.
		return nil
	}

	// The key provider is Computed, so the diff reports it as unknown as soon
	// as the block appears in the configuration. Only the prior state says
	// which key the cluster actually uses.
	state, _ := d.GetChange("encryption_at_rest")
	block, ok := castToNestedBlock(state)
	if !ok {
		return nil
	}

	keyID, _ := block["key_id"].(string)
	provider, _ := block["provider"].(string)
	if keyID == "" || !isCustomerManagedKeyProvider(provider) {
		return nil
	}

	return fmt.Errorf(
		`"key_id" is missing from the "encryption_at_rest" block but the cluster uses customer-managed key %q; `+
			`encryption at rest cannot be changed after creation, so either restore "key_id" or recreate the cluster`,
		keyID,
	)
}

// configuredEncryptionAtRest returns encryption_at_rest[0].key_id from the raw configuration.
// It also reports whether the block is configured at all.
func configuredEncryptionAtRest(config cty.Value) (keyID string, configured bool) {
	if config.IsNull() || !config.IsKnown() {
		// Destroy plan condition.
		return "", false
	}

	blocks := config.GetAttr("encryption_at_rest")
	if blocks.IsNull() || !blocks.IsKnown() || blocks.LengthInt() == 0 {
		return "", false
	}

	value := blocks.Index(cty.NumberIntVal(0)).GetAttr("key_id")
	if value.IsNull() || !value.IsKnown() {
		return "", true
	}

	return value.AsString(), true
}

func resourceClusterCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	scaling, _ := castToNestedBlock(d.Get("scaling"))
	_, hasMinNodes := d.GetOk("min_nodes")
	_, hasNodeType := d.GetOk("node_type")

	if err := validateScaling(hasMinNodes, hasNodeType, scaling); err != nil {
		return err
	}

	if encryptionAtRest, ok := castToNestedBlock(d.Get("encryption_at_rest")); ok {
		enabled, _ := encryptionAtRest["enabled"].(bool)
		configuredKeyID, _ := configuredEncryptionAtRest(d.GetRawConfig())
		if err := validateEncryptionAtRest(d.Get("cloud").(string), enabled, configuredKeyID); err != nil {
			return err
		}
	}

	return validateEncryptionKeyIDNotRemoved(d)
}

func ResourceCluster() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClusterCreate,
		ReadContext:   resourceClusterRead,
		UpdateContext: resourceClusterUpdate,
		DeleteContext: resourceClusterDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(clusterRetryTimeout),
			Update: schema.DefaultTimeout(clusterRetryTimeout),
			Delete: schema.DefaultTimeout(clusterDeleteTimeout),
		},

		CustomizeDiff: resourceClusterCustomizeDiff,

		SchemaVersion: 3,

		StateUpgraders: []schema.StateUpgrader{
			{
				Version: 0,
				Type:    resourceClusterV0().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceClusterUpgradeV0,
			},
			{
				Version: 1,
				Type:    resourceClusterV1().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceClusterUpgradeV1,
			},
			{
				Version: 2,
				Type:    resourceClusterV2().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceClusterUpgradeV2,
			},
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "The computed cluster ID.",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"cloud": {
				Description: "The cloud provider. Accepted values: AWS, GCP.",
				Optional:    true,
				ForceNew:    true,
				Default:     "AWS",
				Type:        schema.TypeString,
			},
			"name": {
				Description: "The name of the cluster.",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"region": {
				Description: "The cloud region to deploy the cluster in (e.g. us-east-1).",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"node_count": {
				Description: "The last retrieved number of nodes.",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"min_nodes": {
				Description: "Minimum number of nodes in the cluster. Required for Standard clusters; must be at least 3 and divisible by 3. " +
					"Must not be set when the scaling block is present, in which case it reads back as `0` and `node_count` reports the number of nodes the cluster currently runs. " +
					"Increasing this value scales the cluster out; decreasing it scales the cluster in. Either operation takes effect immediately on `terraform apply` and does not force cluster re-creation.",
				Optional:         true,
				Type:             schema.TypeInt,
				ConflictsWith:    []string{"scaling"},
				ValidateDiagFunc: validateMinNodesDiag,
			},
			"byoa_id": {
				Description: "The ID of your account (BYOA) in ScyllaDB Cloud (only for AWS).",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeInt,
			},
			"user_api_interface": {
				Description: "The type of user API interface. Valid values are CQL or ALTERNATOR.",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
				Default:     "CQL",
			},
			"alternator_write_isolation": {
				Description: "The write isolation policy. Used only for the ALTERNATOR API interface.",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
				Default:     "only_rmw_uses_lwt",
			},
			"node_type": {
				Description: "The instance type for cluster nodes (e.g. i8g.large). Required for Standard clusters. " +
					"Must not be set when the scaling block is present, in which case it reads back as empty: " +
					"the control plane picks the instance from the scaling policy and changes it as the cluster scales.",
				Optional:      true,
				ForceNew:      true,
				Type:          schema.TypeString,
				ConflictsWith: []string{"scaling"},
			},
			"scaling": {
				Description: "Defines the autoscaling policy for an X Cloud cluster. Mutually exclusive with `node_type` and `min_nodes`. " +
					"When present, the control plane manages scaling automatically based on the policy defined below.",
				Optional:      true,
				Type:          schema.TypeList,
				MaxItems:      1,
				ConflictsWith: []string{"min_nodes", "node_type"},
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"instance_families": {
						Description: `Instance families to use for autoscaling (e.g. ["i8g"]). X Cloud scales within one predefined instance family. ` +
							`Manually restricting the cluster to a narrow set of instance types can limit the effectiveness of the autoscaling engine. ` +
							`Either instance_families or instance_types should be used.`,
						Optional: true,
						Type:     schema.TypeList,
						MinItems: 1,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"instance_types": {
						Description: `Instance types to use for autoscaling (e.g. ["i8g.large", "i8g.xlarge"]). ` +
							`Consider using instance_families instead. Either instance_families or instance_types should be used.`,
						Optional: true,
						Type:     schema.TypeList,
						MinItems: 1,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"storage_policy": {
						Description: "Controls storage-based autoscaling.",
						Optional:    true,
						Type:        schema.TypeList,
						MaxItems:    1,
						Elem: &schema.Resource{Schema: map[string]*schema.Schema{
							"min_gb": {
								Description: "Minimum physical storage, in gigabytes, to keep provisioned across the cluster. " +
									"The cluster will not scale below this threshold. If omitted, ScyllaDB Cloud manages baseline storage dynamically.",
								Required: true,
								Type:     schema.TypeInt,
							},
							"target_utilization": {
								Description: "Target storage utilization as a fraction between 0 and 1 (e.g. 0.75 = 75%). " +
									"The autoscaler adds or removes capacity to maintain this level. Defaults to 0.8. Maximum is 0.9. " +
									"For write-intensive workloads, values below 0.85 are recommended to provide headroom before the autoscaler triggers.",
								Required:         true,
								Type:             schema.TypeFloat,
								ValidateDiagFunc: validateScalingTargetUtilizationDiag,
							},
						}},
					},
					"vcpu_policy": {
						Description: "Controls compute-based autoscaling.",
						Optional:    true,
						Type:        schema.TypeList,
						MaxItems:    1,
						Elem: &schema.Resource{Schema: map[string]*schema.Schema{
							"min": {
								Description: "Minimum vCPU count to maintain across the cluster. The cluster will not scale below this compute baseline. " +
									"If omitted, ScyllaDB Cloud manages compute capacity dynamically.",
								Required: true,
								Type:     schema.TypeInt,
							},
						}},
					},
				}},
			},
			"node_dns_names": {
				Description: "The cluster nodes DNS names.",
				Computed:    true,
				Type:        schema.TypeSet,
				Elem:        schema.TypeString,
				Set:         schema.HashString,
			},
			"node_private_ips": {
				Description: "The cluster nodes private IP addresses.",
				Computed:    true,
				Type:        schema.TypeSet,
				Elem:        schema.TypeString,
				Set:         schema.HashString,
			},
			"cidr_block": {
				Description: "The CIDR block for the cluster network.",
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"scylla_version": {
				Description: "Scylla version. The latest version will be used by default.",
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Type:        schema.TypeString,
			},
			"enable_vpc_peering": {
				Description: "Whether to enable VPC peering for the cluster.",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeBool,
				Default:     true,
			},
			"enable_dns": {
				Description: "Whether to enable DNS for the cluster.",
				Optional:    true,
				ForceNew:    true,
				Type:        schema.TypeBool,
				Default:     true,
			},
			"request_id": {
				Description: "The cluster creation request ID.",
				Computed:    true,
				Type:        schema.TypeInt,
			},
			"datacenter": {
				Description: "The computed cluster datacenter name.",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"status": {
				Description: "The cluster status.",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"ca_certificate": {
				Description: "The PEM-encoded CA certificate used to verify TLS (client-to-node encrypted) " +
					"connections to the cluster. Empty if in-transit encryption is not enabled.",
				Computed: true,
				Type:     schema.TypeString,
			},
			"node_disk_size": {
				Description: "The disk size in gigabytes of the node. " +
					"Must not be set when the scaling block is present, in which case it reads back as `0`: " +
					"the control plane picks the instance from the scaling policy and changes it as the cluster scales.",
				ForceNew:      true,
				Optional:      true,
				Computed:      true,
				Type:          schema.TypeInt,
				ConflictsWith: []string{"scaling"},
			},
			"availability_zone_ids": {
				Description: `Availability zone IDs where cluster nodes are provisioned. ` +
					`Provide exactly 3 distinct AZ IDs (e.g. ["use1-az1", "use1-az4", "use1-az5"]). ` +
					`If omitted, zones are selected automatically. After refreshing state with terraform refresh, ` +
					`you can read back the IDs that were assigned.`,
				Optional: true,
				Computed: true,
				ForceNew: true,
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"backup_retention_days": {
				Description: "The number of days to retain backups after deleting the cluster between 0 and 60. " +
					"If set to 0, backups are deleted immediately. " +
					"Defaults to 1 to prevent accidental data loss.",
				Optional:         true,
				Type:             schema.TypeInt,
				Default:          1,
				ValidateDiagFunc: validateBackupRetentionDaysDiag,
			},
			"encryption_at_rest": {
				Description: "Configures database-level encryption at rest. The key provider is derived from " +
					"the `cloud` attribute. Encryption at rest can only be configured when the cluster is " +
					"created, so changing any field in this block replaces the cluster. " +
					"New clusters are encrypted with a ScyllaDB-managed key by default. The block is " +
					"needed to opt out with `enabled = false` or to point at a customer-managed key. " +
					"Existing clusters are never modified.",
				Optional: true,
				Computed: true,
				ForceNew: true,
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"enabled": {
						Description: "Whether the cluster data is encrypted at rest. Defaults to true; set it " +
							"to false to create the cluster unencrypted.",
						Optional: true,
						ForceNew: true,
						Default:  true,
						Type:     schema.TypeBool,
					},
					"key_id": {
						Description: "The ID of a customer-managed key pre-created in the ScyllaDB Cloud portal " +
							"(e.g. `key-deadbeef`). Leave it unset to let ScyllaDB Cloud manage the key.",
						Optional: true,
						Computed: true,
						ForceNew: true,
						Type:     schema.TypeString,
					},
					"provider": {
						Description: "The key provider resolved by the API: `scylla-aws` or `scylla-gcp` for a " +
							"ScyllaDB-managed key, `aws` or `gcp` for a customer-managed one. " +
							"Empty if encryption at rest is not enabled.",
						Computed: true,
						Type:     schema.TypeString,
					},
				}},
			},
		},
	}
}

func resourceClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var (
		scyllaClient         = meta.(*scylla.Client)
		clusterCreateRequest = &model.ClusterCreateRequest{
			ClusterName:          d.Get("name").(string),
			BroadcastType:        "PRIVATE",
			ReplicationFactor:    3,
			UserAPIInterface:     d.Get("user_api_interface").(string),
			EnableDNSAssociation: d.Get("enable_dns").(bool),
			Placement:            "true",
		}
		cloud                        = d.Get("cloud").(string)
		cidr, cidrOK                 = d.GetOk("cidr_block")
		byoa, byoaOK                 = d.GetOk("byoa_id")
		region                       = d.Get("region").(string)
		nodeType, nodeTypeOK         = d.GetOk("node_type")
		scaling                      *model.Scaling
		version, versionOK           = d.GetOk("scylla_version")
		enableVpcPeering             = d.Get("enable_vpc_peering").(bool)
		nodeDiskSize, nodeDiskSizeOK = d.GetOk("node_disk_size")
	)

	if !enableVpcPeering {
		clusterCreateRequest.BroadcastType = "PUBLIC"
	}

	if clusterCreateRequest.UserAPIInterface == "ALTERNATOR" {
		clusterCreateRequest.AlternatorWriteIsolation = d.Get("alternator_write_isolation").(string)
	}

	if byoaOK {
		clusterCreateRequest.AccountCredentialID = int64(byoa.(int))
	}

	if !cidrOK {
		cidr = "172.31.0.0/16"
		_ = d.Set("cidr_block", cidr)
	}

	cloudProvider := scyllaClient.Meta.ProviderByName(cloud)
	if cloudProvider == nil {
		return diag.Errorf(`unrecognized value %q for "cloud" attribute`, cloud)
	}

	clusterCreateRequest.CidrBlock = cidr.(string)

	clusterCreateRequest.CloudProviderID = cloudProvider.CloudProvider.ID

	mr := cloudProvider.RegionByName(region)
	if mr != nil {
		clusterCreateRequest.RegionID = mr.ID
	} else {
		return diag.Errorf(`unrecognized value %q for "region" attribute`, region)
	}

	instances, err := scyllaClient.ListCloudProviderInstancesPerRegion(ctx, cloudProvider.CloudProvider.ID, mr.ID)
	if err != nil {
		return diag.Errorf("failed to list cloud provider instances for region %q: %s", region, err)
	}

	clusterCreateRequest.EncryptionAtRest, err = expandEncryptionAtRest(cloud, d.Get("encryption_at_rest"))
	if err != nil {
		return diag.FromErr(err)
	}

	scaling, err = expandScaling(d.Get("scaling"), region, instances, cloudProvider)
	if err != nil {
		return diag.FromErr(err)
	}

	if scaling != nil {
		clusterCreateRequest.Scaling = scaling
	} else {
		minNodes := d.Get("min_nodes").(int)
		clusterCreateRequest.NumberOfNodes = int64(minNodes)
	}

	var mi *model.CloudProviderInstance
	if scaling == nil {
		nodeTypeStr := nodeType.(string)
		if nodeDiskSizeOK {
			if mi = cloudProvider.InstanceByNameAndDiskSizeFromInstances(nodeTypeStr, nodeDiskSize.(int), instances); mi == nil {
				return diag.Errorf(
					`unrecognized value combination: %q for "node_type" and %d for "node_disk_size" attributes`,
					nodeTypeStr,
					nodeDiskSize,
				)
			}
		} else {
			if mi = cloudProvider.InstanceByNameFromInstances(nodeTypeStr, instances); mi == nil {
				return diag.Errorf(`unsupported node_type %q in region %s`, nodeTypeStr, mr.ExternalID)
			}
		}

		clusterCreateRequest.InstanceID = mi.ID
	} else if nodeDiskSizeOK || nodeTypeOK {
		return diag.Errorf(`"node_type" and "node_disk_size" are not supported when the "scaling" block is configured`)
	}

	// Handle availability zone IDs
	if azIDs, ok := d.GetOk("availability_zone_ids"); ok {
		// Figure out the cloud account ID; it's either BYOA or Scylla Account.
		// If cloudAccountID is 0, we look up the active cloud account owned by Scylla.
		cloudAccountID := clusterCreateRequest.AccountCredentialID
		if cloudAccountID == 0 {
			cloudAccounts, err := scyllaClient.ListCloudAccounts(ctx)
			if err != nil {
				return diag.Errorf("failed to list cloud accounts: %s", err)
			}

			ca := model.FindScyllaCloudAccount(cloudAccounts, cloudProvider.CloudProvider.ID)
			if ca == nil {
				return diag.Errorf(
					"no active Scylla-owned cloud account found for cloud provider %q (ID %d)",
					cloud, cloudProvider.CloudProvider.ID,
				)
			}
			cloudAccountID = ca.ID
		}

		azIDsSet := azIDs.(*schema.Set)

		var azIDList []string
		for _, v := range azIDsSet.List() {
			azIDList = append(azIDList, v.(string))
		}
		slices.Sort(azIDList)

		if err := validateAvailabilityZoneIDs(ctx, scyllaClient, cloudAccountID, mr.ID, azIDList); err != nil {
			return diag.FromErr(err)
		}

		clusterCreateRequest.AvailabilityZoneIDs = azIDList
	}

	if !versionOK {
		clusterCreateRequest.ScyllaVersionID = scyllaClient.Meta.ScyllaVersions.DefaultScyllaVersionID
	} else if mv := scyllaClient.Meta.VersionByName(version.(string)); mv != nil {
		clusterCreateRequest.ScyllaVersionID = mv.ID
	} else {
		return diag.Errorf(`unrecognized value %q for "scylla_version" attribute`, version)
	}

	_, encryptionConfigured := configuredEncryptionAtRest(d.GetRawConfig())

	cr, warns, err := createClusterWithEncryptionFallback(ctx, scyllaClient, clusterCreateRequest, encryptionConfigured)
	if err != nil {
		return diag.Errorf("failed to create a cluster request: %s", err)
	}

	if err := WaitForClusterRequestID(ctx, scyllaClient, cr.ID); err != nil {
		return diag.Errorf("failed to wait for request %d creating cluster %d: %s", cr.ID, cr.ClusterID, err)
	}

	cluster, err := scyllaClient.GetCluster(ctx, cr.ClusterID)
	if err != nil {
		return diag.Errorf("failed to read cluster %d: %s", cr.ClusterID, err)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	// The instance the cluster runs on is only tracked for Standard clusters.
	// X Cloud picks it from the scaling policy and changes it as the cluster
	// scales, so resolving it here would only produce a value setClusterKVs
	// discards.
	instanceExternalID := ""
	if !hasScaling(cluster) && cluster.Datacenter.InstanceID != 0 {
		i := cloudProvider.InstanceByIDFromInstances(cluster.Datacenter.InstanceID, instances)
		if i == nil {
			return diag.Errorf("unexpected instance ID for cluster %d: %d", cluster.ID, cluster.Datacenter.InstanceID)
		}
		instanceExternalID = i.ExternalID
	}
	caCert, certWarns := fetchCACertificate(ctx, scyllaClient, cluster.ID, "")
	warns = append(warns, certWarns...)

	err = setClusterKVs(d, cluster, cloudProvider.CloudProvider.Name, instanceExternalID, caCert, instances, cloudProvider)
	if err != nil {
		return diag.Errorf("failed to set cluster values for cluster %d: %s", cluster.ID, err)
	}

	d.SetId(strconv.Itoa(int(cr.ClusterID)))
	_ = d.Set("request_id", cr.ID)

	return warns
}

func resourceClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	scyllaClient := meta.(*scylla.Client)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	reqs, err := scyllaClient.ListClusterRequest(
		ctx,
		clusterID,
		scylla.ListClusterRequestParams{Type: "CREATE_CLUSTER"},
	)
	switch {
	case scylla.IsDeletedErr(err):
		_ = d.Set("status", "DELETED")
		return nil
	case err != nil:
		return diag.Errorf("failed to list cluster requests for cluster %d: %s", clusterID, err)
	case len(reqs) != 1:
		return diag.Errorf("unexpected number of cluster requests; expected 1, got %d: %+v", len(reqs), reqs)
	}
	_ = d.Set("request_id", reqs[0].ID)

	if reqs[0].Status != "COMPLETED" {
		if err := WaitForClusterRequestID(ctx, scyllaClient, reqs[0].ID); err != nil {
			return diag.Errorf("failed to wait for cluster request %d: %s", reqs[0].ID, err)
		}
	}

	cluster, err := scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("failed to read cluster %d: %s", clusterID, err)
	}

	p := scyllaClient.Meta.ProviderByID(cluster.CloudProviderID)
	if p == nil {
		return diag.Errorf("unexpected cloud provider %d for cluster %d", cluster.CloudProviderID, cluster.ID)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	var instanceExternalID string
	instances, err := scyllaClient.ListCloudProviderInstancesPerRegion(ctx, cluster.CloudProviderID, cluster.Region.ID)
	if err != nil {
		return diag.Errorf("failed to list cloud provider instances for region %q: %s", cluster.Region.ExternalID, err)
	}
	// Standard clusters only; see the matching comment in resourceClusterCreate.
	if !hasScaling(cluster) && cluster.Datacenter.InstanceID != 0 {
		i := p.InstanceByIDFromInstances(cluster.Datacenter.InstanceID, instances)
		if i == nil {
			return diag.Errorf("unexpected instance ID for cluster %d: %d", cluster.ID, cluster.Datacenter.InstanceID)
		}
		instanceExternalID = i.ExternalID
	}
	caCert, warns := fetchCACertificate(ctx, scyllaClient, clusterID, d.Get("ca_certificate").(string))
	err = setClusterKVs(d, cluster, p.CloudProvider.Name, instanceExternalID, caCert, instances, p)
	if err != nil {
		return diag.Errorf("failed to set cluster values for cluster %d: %s", cluster.ID, err)
	}

	return warns
}

// fetchCACertificate retrieves the cluster's CA certificate. Clusters without
// in-transit encryption (error 041201) yield an empty string. Any other
// failure is downgraded to a warning and prev is returned, so a misbehaving
// certificate endpoint never breaks plan/refresh or leaks a created cluster.
func fetchCACertificate(ctx context.Context, c *scylla.Client, clusterID int64, prev string) (string, diag.Diagnostics) {
	cert, err := c.GetClusterCertificate(ctx, clusterID)
	switch {
	case scylla.IsEncryptionDisabledErr(err):
		return "", nil
	case err != nil:
		return prev, diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("Unable to retrieve CA certificate for cluster %d", clusterID),
			Detail:   err.Error(),
		}}
	}
	return cert.Content, nil
}

func setClusterKVs(d *schema.ResourceData, cluster *model.Cluster, providerName, instanceExternalID, caCert string, instances []model.CloudProviderInstance, cloudProvider *scylla.CloudProvider) error {
	_ = d.Set("cluster_id", cluster.ID)
	_ = d.Set("name", cluster.ClusterName)
	_ = d.Set("cloud", providerName)
	_ = d.Set("region", cluster.Region.ExternalID)

	nodeCount := len(model.NodesByStatus(cluster.Nodes, "ACTIVE"))
	_ = d.Set("node_count", nodeCount)

	if hasScaling(cluster) {
		// min_nodes, node_type and node_disk_size describe a Standard cluster.
		// For X Cloud the scaling policy owns them and the control plane keeps
		// changing the instance as the cluster scales, so tracking them would
		// report drift after every scaling event. The SDK cannot store null in
		// a primitive, so they read back as 0 and "".
		_ = d.Set("min_nodes", nil)
		_ = d.Set("node_type", nil)
		_ = d.Set("node_disk_size", nil)
		scaling, err := flattenScaling(cluster.Datacenters[0].Scaling, instances, cloudProvider)
		if err != nil {
			return err
		}
		_ = d.Set("scaling", scaling)
	} else if minNodes, ok := d.GetOk("min_nodes"); !ok {
		_ = d.Set("scaling", []map[string]interface{}{})
		_ = d.Set("min_nodes", nodeCount)
	} else if minNodes.(int) > nodeCount {
		_ = d.Set("scaling", []map[string]interface{}{})
		// If the cluster was scaled in outside of the Terraform,
		// set min_nodes to its new value. It should result in
		// scale out as it will diverge from what's in the .tf
		// file, which is a true desired state.
		//
		// This covers the following scenario:
		//  - create a cluster using TF with min_nodes = 6,
		//  - scale-in using API
		//  - run "tf apply"
		// Expectations: min_nodes should be updated and the apply should result in scale-out.
		_ = d.Set("min_nodes", nodeCount)
	} else {
		_ = d.Set("scaling", []map[string]interface{}{})
	}

	_ = d.Set("user_api_interface", cluster.UserAPIInterface)

	if !hasScaling(cluster) {
		_ = d.Set("node_type", instanceExternalID)
	}
	_ = d.Set("node_dns_names", model.NodesDNSNames(cluster.Nodes))
	_ = d.Set("node_private_ips", model.NodesPrivateIPs(cluster.Nodes))
	_ = d.Set("cidr_block", cluster.Datacenter.CIDRBlock)
	_ = d.Set("scylla_version", cluster.ScyllaVersion.Version)
	_ = d.Set("enable_vpc_peering", !strings.EqualFold(cluster.BroadcastType, "PUBLIC"))
	_ = d.Set("enable_dns", cluster.DNS)
	_ = d.Set("datacenter", cluster.Datacenter.Name)
	_ = d.Set("status", cluster.Status)
	_ = d.Set("ca_certificate", caCert)
	_ = d.Set("encryption_at_rest", flattenEncryptionAtRest(cluster.EncryptionAtRest))

	if cluster.UserAPIInterface == "ALTERNATOR" {
		_ = d.Set("alternator_write_isolation", cluster.AlternatorWriteIsolation)
	}

	if id := cluster.Datacenter.AccountCloudProviderCredentialID; id >= 1000 {
		_ = d.Set("byoa_id", id)
	}

	if !hasScaling(cluster) && cluster.Instance != nil {
		_ = d.Set("node_disk_size", cluster.Instance.TotalStorage)
	}

	azIDs := cluster.Datacenter.AvailabilityZoneIDs()
	if azIDs == nil {
		// Prevent stale data in case the new value is empty or missing.
		azIDs = []string{}
	}
	_ = d.Set("availability_zone_ids", azIDs)

	return nil
}

func resourceClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	scyllaClient := meta.(*scylla.Client)
	if d.HasChange("scaling") {
		return resourceClusterUpdateScaling(ctx, d, scyllaClient)
	}

	if d.HasChange("min_nodes") {
		return resourceClusterUpdateMinNodes(ctx, d, meta, scyllaClient)
	}

	return nil
}

func resourceClusterUpdateMinNodes(ctx context.Context, d *schema.ResourceData, meta interface{}, scyllaClient *scylla.Client) diag.Diagnostics {
	// There are three scenarios:
	// - scale-out: newMinNodes > oldMinNodes
	// - scale-in: newMinNodes < oldMinNodes
	// - no-op: newMinNodes == oldMinNodes
	//
	// The no-op case is already handled above by checking `d.HasChange()`.
	//
	// Scale-out is easy: we just request more nodes.
	//
	// Scale-in is more complicated. It may happen that it's currently
	// not possible, because after the scale-in there would be not enough
	// disk space. Such an update should fail, meaning that the value
	// of min_nodes should not be changed. A user can try again later.
	//
	// Note that it's not possible to update min_nodes and defer
	// scale-in until later, e.g., when there is enough disk space,
	// because min_nodes controls the resize API behavior rather
	// than controlling the desired state. If such a behavior is needed,
	// please consider X Cloud. More:
	// https://www.scylladb.com/product/scylladb-xcloud/

	oldMinNodesI, newMinNodesI := d.GetChange("min_nodes")
	oldMinNodes, newMinNodes := oldMinNodesI.(int), newMinNodesI.(int)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	tflog.Debug(ctx, "Updating cluster min_nodes", map[string]interface{}{
		"cluster_id": clusterID,
		"old":        oldMinNodes,
		"new":        newMinNodes,
	})

	cluster, err := scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("failed to get the cluster with ID %d: %s", clusterID, err)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	// Resize will fail if there is any ongoing cluster request.
	if err := WaitForNoInProgressRequests(ctx, scyllaClient, cluster.ID); err != nil {
		return diag.Errorf("failed waiting for no in-progress cluster requests for cluster %d: %s", cluster.ID, err)
	}

	// Re-fetch the cluster to get the current node count after waiting,
	// as it may have changed during the wait period.
	cluster, err = scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil // cluster was deleted
		}
		return diag.Errorf("failed to get the cluster with ID %d: %s", clusterID, err)
	}

	curNodesCount := len(model.NodesByStatus(cluster.Nodes, "ACTIVE"))

	if newMinNodes == curNodesCount {
		tflog.Debug(ctx, "Current number of nodes equals min_nodes; return", map[string]interface{}{
			"cluster_id":      clusterID,
			"cur_nodes_count": curNodesCount,
			"new_min_nodes":   newMinNodes,
		})
		return resourceClusterRead(ctx, d, meta)
	}

	tflog.Debug(ctx, "min_nodes is different than current number of nodes; proceed with resize", map[string]interface{}{
		"cluster_id":      clusterID,
		"cur_nodes_count": curNodesCount,
		"new_min_nodes":   newMinNodes,
	})

	resizeRequest, err := scyllaClient.ResizeCluster(
		ctx,
		cluster.ID,
		cluster.Datacenter.ID,
		cluster.Datacenter.InstanceID,
		newMinNodes,
	)
	if err != nil {
		return diag.Errorf("error resizing cluster: %s", err)
	}

	if err := WaitForClusterRequestID(ctx, scyllaClient, resizeRequest.ID); err != nil {
		return diag.Errorf(
			"failed waiting for the cluster resize with ID %d for the cluster %d: %s",
			resizeRequest.ID, cluster.ID, err,
		)
	}

	return resourceClusterRead(ctx, d, meta)
}

func resourceClusterUpdateScaling(ctx context.Context, d *schema.ResourceData, scyllaClient *scylla.Client) diag.Diagnostics {
	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	cluster, err := scyllaClient.GetCluster(ctx, clusterID)
	if err != nil {
		if scylla.IsClusterDeletedErr(err) {
			d.SetId("")
			return nil
		}
		return diag.Errorf("failed to get the cluster with ID %d: %s", clusterID, err)
	}

	if n := len(cluster.Datacenters); n != 1 {
		return diag.Errorf("clusters without datacenter or multi-datacenter clusters are not currently supported (found %d datacenters)", n)
	}

	instances, err := scyllaClient.ListCloudProviderInstancesPerRegion(ctx, cluster.CloudProviderID, cluster.Region.ID)
	if err != nil {
		return diag.Errorf("failed to list cloud provider instances: %s", err)
	}

	cloudProvider := scyllaClient.Meta.ProviderByID(cluster.CloudProviderID)
	if cloudProvider == nil {
		return diag.Errorf("unexpected cloud provider %d for cluster %d", cluster.CloudProviderID, cluster.ID)
	}

	desiredScaling, err := expandScaling(d.Get("scaling"), cluster.Region.ExternalID, instances, cloudProvider)
	if err != nil {
		return diag.FromErr(err)
	}
	if desiredScaling == nil {
		return diag.Errorf("scaling block must be configured for scaling updates")
	}
	remoteScaling := cluster.Datacenters[0].Scaling

	if !hasScaling(cluster) {
		return diag.Errorf("scaling updates are supported only for X Cloud clusters")
	}

	if isScalingEqual(desiredScaling, remoteScaling) {
		return resourceClusterRead(ctx, d, scyllaClient)
	}

	request, err := scyllaClient.UpdateDcScalingPolicy(ctx, cluster.ID, cluster.Datacenter.ID, desiredScaling)
	if err != nil {
		var apiErr *scylla.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "041008" {
			return diag.Errorf("X-Cloud clusters do not support manual resizing. Use the scaling block to adjust capacity policies.")
		}
		return diag.Errorf("failed to update cluster scaling: %s", err)
	}

	if request == nil || request.ID == 0 {
		return diag.Errorf("failed to update cluster scaling: missing cluster request ID in response")
	}

	if err := WaitForClusterRequestID(ctx, scyllaClient, request.ID); err != nil {
		return diag.Errorf("failed waiting for scaling update request %d for cluster %d: %s", request.ID, cluster.ID, err)
	}

	return resourceClusterRead(ctx, d, scyllaClient)
}

func resourceClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)

	clusterID, diags := parseClusterID(d)
	if diags != nil {
		return diags
	}

	name, ok := d.GetOk("name")
	if !ok {
		return diag.Errorf("failed to read the cluster name from the resource")
	}

	backupRetentionDays := d.Get("backup_retention_days").(int)

	r, err := c.DeleteCluster(ctx, clusterID, name.(string), backupRetentionDays)
	if err != nil {
		if scylla.IsDeletedErr(err) {
			return nil // cluster was already deleted
		}
		return diag.Errorf("failed to delete the cluster: %s", err)
	}

	if !strings.EqualFold(r.Status, "QUEUED") && !strings.EqualFold(r.Status, "IN_PROGRESS") && !strings.EqualFold(r.Status, "COMPLETED") {
		return diag.Errorf(
			"delete cluster returned unknown status %q for the cluster request %d for the cluster %d",
			r.Status, r.ID, clusterID,
		)
	}

	return nil
}

// WaitForClusterRequestID returns only after the cluster request is completed or failed.
func WaitForClusterRequestID(ctx context.Context, c *scylla.Client, requestID int64) error {
	t := time.NewTicker(clusterPollInterval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			r, err := c.GetClusterRequest(ctx, requestID)
			if err != nil {
				return fmt.Errorf("failed to get cluster request with ID %d: %w", requestID, err)
			}

			if strings.EqualFold(r.Status, "COMPLETED") {
				return nil
			}
			if strings.EqualFold(r.Status, "FAILED") {
				return fmt.Errorf("cluster request ID %d failed", r.ID)
			}
			if strings.EqualFold(r.Status, "QUEUED") || strings.EqualFold(r.Status, "IN_PROGRESS") {
				continue
			}

			return fmt.Errorf("unknown cluster request status: %s", r.Status)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForNoInProgressRequests waits until there are no requests in progress.
func WaitForNoInProgressRequests(ctx context.Context, c *scylla.Client, clusterID int64) error {
	t := time.NewTicker(clusterPollInterval)
	defer t.Stop()

	checkAllClear := func() (bool, error) {
		for _, status := range []string{"IN_PROGRESS"} {
			reqs, err := c.ListClusterRequest(
				ctx,
				clusterID,
				scylla.ListClusterRequestParams{Status: status},
			)
			if err != nil {
				return false, err
			}
			if len(reqs) > 0 {
				return false, nil
			}
		}
		return true, nil
	}

	// Check immediately before waiting for the first tick.
	allClear, err := checkAllClear()
	if err != nil {
		return err
	}
	if allClear {
		return nil
	}

	for {
		select {
		case <-t.C:
			allClear, err := checkAllClear()
			if err != nil {
				return err
			}
			if allClear {
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func parseClusterID(d *schema.ResourceData) (int64, diag.Diagnostics) {
	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return 0, diag.Errorf("failed to parse a cluster ID %q: %s", d.Id(), err)
	}
	return clusterID, nil
}

// validateAvailabilityZoneIDs validates that the provided AZ IDs are valid for the given region.
func validateAvailabilityZoneIDs(ctx context.Context, c *scylla.Client, cloudAccountID, regionID int64, azIDs []string) error {
	if l := len(azIDs); l < 1 || l > 3 {
		return fmt.Errorf("at least 1 and at most 3 availability zone IDs are required, got %d", l)
	}

	// Check for duplicate AZ IDs.
	seen := make(map[string]struct{}, len(azIDs))
	var duplicates []string
	for _, azID := range azIDs {
		if _, ok := seen[azID]; ok {
			duplicates = append(duplicates, azID)
		} else {
			seen[azID] = struct{}{}
		}
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate availability zone IDs are not allowed: %v", duplicates)
	}

	// Validate available AZ IDs.
	availableAZs, err := c.ListAvailabilityZoneIDs(ctx, cloudAccountID, regionID)
	if err != nil {
		return fmt.Errorf("failed to list availability zones for region: %w", err)
	}

	availableSet := make(map[string]struct{}, len(availableAZs))
	for _, az := range availableAZs {
		availableSet[az] = struct{}{}
	}

	var invalidAZs []string
	for _, azID := range azIDs {
		if _, ok := availableSet[azID]; !ok {
			invalidAZs = append(invalidAZs, azID)
		}
	}

	if len(invalidAZs) > 0 {
		return fmt.Errorf(
			"invalid availability zone IDs %v; available AZ IDs for this region are: %v",
			invalidAZs,
			availableAZs,
		)
	}

	return nil
}
