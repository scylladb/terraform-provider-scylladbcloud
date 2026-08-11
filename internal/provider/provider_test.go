package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkterraform "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	providercluster "github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	v2scylla "github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/v2"
)

var provider *schema.Provider = New(context.Background())

var protoV5ProviderFactories map[string]func() (tfprotov5.ProviderServer, error) = protoV5ProviderFactoriesInit(context.Background())

func protoV5ProviderFactoriesInit(ctx context.Context) map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"scylladbcloud": func() (tfprotov5.ProviderServer, error) {
			providerServerFactory, _, err := ProtoV5ProviderServerFactory(ctx)
			if err != nil {
				return nil, err
			}
			return providerServerFactory(), nil
		},
	}
}

func TestAccScyllaDBCloudCluster_basicAWS(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  scylla_version        = "2026.1.1"
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
  availability_zone_ids = ["use1-az2", "use1-az4", "use1-az6"]
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("availability_zone_ids"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("use1-az2"),
							knownvalue.StringExact("use1-az4"),
							knownvalue.StringExact("use1-az6"),
						}),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("scylla_version"),
						knownvalue.StringExact("2026.1.1"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("ca_certificate"),
						knownvalue.StringRegexp(regexp.MustCompile(`^-----BEGIN CERTIFICATE-----`)),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
					func(s *terraform.State) error {
						if cluster.ScyllaVersion == nil {
							return fmt.Errorf("cluster ScyllaVersion is nil")
						}
						if cluster.ScyllaVersion.Version != "2026.1.1" {
							return fmt.Errorf("expected scylla_version %q, got %q",
								"2026.1.1", cluster.ScyllaVersion.Version)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_xcloudAWS(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("xcloud-aws")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
  availability_zone_ids = ["use1-az2", "use1-az4", "use1-az6"]
  scaling {
    instance_families = ["i4i"]
    storage_policy {
      min_gb             = 500
      target_utilization = 0.75
    }
    vcpu_policy {
      min = 8
    }
  }
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("scaling"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectPartial(
								map[string]knownvalue.Check{
									"instance_families": knownvalue.ListExact(
										[]knownvalue.Check{knownvalue.StringExact("i4i")},
									),
								},
							),
						}),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("availability_zone_ids"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("use1-az2"),
							knownvalue.StringExact("use1-az4"),
							knownvalue.StringExact("use1-az6"),
						}),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicAWSSingleAZ(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws-single-az")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
  availability_zone_ids = ["use1-az2"]
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("availability_zone_ids"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("use1-az2"),
						}),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicAWSScaleOut(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws-scale-out")

	var cluster model.Cluster

	clusterIDCompare := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("cluster_id"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 6
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
}`, resourceName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction("scylladbcloud_cluster.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("cluster_id"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(6),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(6),
					),
				},
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_scaleOutFromOutside(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws-scale-out-outside")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				PreConfig: func() {
					client := getClientFromProvider(provider)

					err := providercluster.WaitForNoInProgressRequests(ctx, client, cluster.ID)
					require.NoError(t, err)

					req, err := client.ResizeCluster(ctx, cluster.ID, cluster.Datacenter.ID, cluster.Datacenter.InstanceID, 6)
					require.NoError(t, err)

					err = providercluster.WaitForClusterRequestID(ctx, client, req.ID)
					require.NoError(t, err)
				},
				RefreshState: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicGCP(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-gcp")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "GCP"
  region                = "us-central1"
  node_type             = "n2d-highmem-2"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicGCPBYOA(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-gcp-byoa")

	byoaID := envGCPBYOAID()
	if byoaID == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID must be set for this test")
	}

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "GCP"
  region                = "us-central1"
  node_type             = "n2d-highmem-2"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
  byoa_id              = %[2]s
}`, resourceName, byoaID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_backupRetentionDefault(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("backup-retention-default")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name       = %[1]q
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  min_nodes  = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("backup_retention_days"),
						knownvalue.Int32Exact(1),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

// testAccEncryptionAtRestConfig renders a minimal cluster whose
// encryption_at_rest block holds the single attribute assignment.
func testAccEncryptionAtRestConfig(name, cloud, region, nodeType, attribute string) string {
	return fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = %[2]q
  region                = %[3]q
  node_type             = %[4]q
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0

  encryption_at_rest {
    %[5]s
  }
}`, name, cloud, region, nodeType, attribute)
}

// testAccEncryptionAtRestEnabledConfig renders a cluster that enabled or disables encryption at rest.
func testAccEncryptionAtRestEnabledConfig(name, cloud, region, nodeType string, enabled bool) string {
	return testAccEncryptionAtRestConfig(name, cloud, region, nodeType,
		fmt.Sprintf("enabled = %t", enabled))
}

// testAccEncryptionAtRestKeyConfig renders a cluster encrypted with the given
// customer-managed key. It leaves "enabled" out on purpose, so that the test
// also covers its default.
func testAccEncryptionAtRestKeyConfig(name, cloud, region, nodeType, keyID string) string {
	return testAccEncryptionAtRestConfig(name, cloud, region, nodeType,
		fmt.Sprintf("key_id = %q", keyID))
}

func TestAccScyllaDBCloudCluster_encryptionAtRestEnabled(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-enabled")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccEncryptionAtRestEnabledConfig(resourceName, "AWS", "us-east-1", "i3.large", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("enabled"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("provider"),
						knownvalue.StringExact("scylla-aws"),
					),
					// The API returns a key ID even for ScyllaDB-managed keys.
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("key_id"),
						knownvalue.StringRegexp(regexp.MustCompile(`^key-`)),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				// Encryption at rest is create-time only, so any change must
				// plan a replacement rather than an in-place update. PlanOnly
				// keeps this assertion from costing a second real cluster.
				Config:             testAccEncryptionAtRestEnabledConfig(resourceName, "AWS", "us-east-1", "i3.large", false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPreRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"scylladbcloud_cluster.test",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_encryptionAtRestDisabled(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-disabled")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccEncryptionAtRestEnabledConfig(resourceName, "AWS", "us-east-1", "i3.large", false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("enabled"),
						knownvalue.Bool(false),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("provider"),
						knownvalue.StringExact(""),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("key_id"),
						knownvalue.StringExact(""),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

// TestAccScyllaDBCloudCluster_encryptionAtRestGCP requires the enable_ear_gcp
// feature flag to be on in the target environment.
func TestAccScyllaDBCloudCluster_encryptionAtRestGCP(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-gcp")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccEncryptionAtRestEnabledConfig(resourceName, "GCP", "us-central1", "n2d-highmem-2", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("provider"),
						knownvalue.StringExact("scylla-gcp"),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

// TestAccScyllaDBCloudCluster_encryptionAtRestCMK needs a customer-managed key
// created up front in the ScyllaDB Cloud portal; there is no public keys API.
func TestAccScyllaDBCloudCluster_encryptionAtRestCMK(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-cmk")

	keyID := envEARKeyID()
	if keyID == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_EAR_KEY_ID must be set for this test")
	}

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccEncryptionAtRestKeyConfig(resourceName, "AWS", "us-east-1", "i3.large", keyID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("enabled"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("provider"),
						knownvalue.StringExact("aws"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("key_id"),
						knownvalue.StringExact(keyID),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_migrationV1ToV2(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws-migration-v1-to-v2")

	var cluster model.Cluster

	nodeCountCompare := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"scylladbcloud": {
						// 1.9 is the last version that uses the v1 schema
						// that is node_count instead of min_nodes.
						//
						// Note: Be careful about overwritting the provider with
						// a local build in .terraformrc. If you do that,
						// it will overwrite the selected version in this
						// test case.
						VersionConstraint: "1.9",
						Source:            "scylladb/scylladbcloud",
					},
				},
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name       = %[1]q
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  node_count = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					nodeCountCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int32Exact(3),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
}`, resourceName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					nodeCountCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("min_nodes"),
						knownvalue.Int32Exact(3),
					),
				},
			},
		},
	})
}

func TestTraceOrNew(t *testing.T) {
	t.Run("configured trace is preserved", func(t *testing.T) {
		trace, err := traceOrNew("explicit")
		require.NoError(t, err)
		require.Equal(t, "explicit", trace)
	})

	t.Run("missing trace is generated", func(t *testing.T) {
		trace, err := traceOrNew("")
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(trace, v2scylla.TracePrefix),
			"trace %q missing prefix %q", trace, v2scylla.TracePrefix)
	})
}

func TestProviderTraceEnvDefault(t *testing.T) {
	t.Setenv("SCYLLADB_CLOUD_TRACE", "from-env")

	p := New(context.Background())

	require.Equal(t, "from-env", p.Schema["trace"].Default)
}

func TestProviderTraceValidation(t *testing.T) {
	p := New(context.Background())
	validate := p.Schema["trace"].ValidateDiagFunc

	require.Nil(t, validate("valid-trace", cty.Path{}))

	diags := validate("bad\r\ntrace", cty.Path{})
	require.Len(t, diags, 1)
	require.Equal(t, diag.Error, diags[0].Severity)
	require.Equal(t, "invalid trace value", diags[0].Summary)
}

var configureProviderOnce sync.Once

func testAccPreCheck(t *testing.T) {
	// Validate that required environment variables are set.
	if v := envToken(); v == "" {
		t.Fatal("SCYLLADB_CLOUD_TOKEN must be set for acceptance tests")
	}
	if v := envEndpoint(); v == "" {
		t.Fatal("SCYLLADB_CLOUD_ENDPOINT must be set for acceptance tests")
	}

	configureProviderOnce.Do(func() {
		diags := provider.Configure(context.Background(), sdkterraform.NewResourceConfigRaw(nil))

		for _, d := range diags {
			if d.Severity == diag.Error {
				panic(d.Summary)
			}
		}
	})
}

func testAccCheckScyllaDBCloudClusterExists(
	ctx context.Context,
	resourceName string,
	cluster *model.Cluster,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getClientFromProvider(provider)

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return errors.Errorf("resource %q not found", resourceName)
		}

		clusterID, err := parseClusterIDFromResourceID(rs.Primary.ID)
		if err != nil {
			return err
		}

		clusterResponse, err := client.GetCluster(ctx, clusterID)
		if err != nil {
			return errors.Wrapf(err, "error retrieving cluster %d", clusterID)
		}

		*cluster = *clusterResponse

		return nil
	}
}

func testAccCheckScyllaDBCloudClusterDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := getClientFromProvider(provider)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "scylladbcloud_cluster" {
				continue
			}

			clusterID, err := parseClusterIDFromResourceID(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = client.GetCluster(ctx, clusterID)
			if err == nil {
				return errors.Errorf("cluster %d still exists", clusterID)
			}
		}

		return nil
	}
}

func envGCPBYOAID() string {
	return os.Getenv("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID")
}

func envEARKeyID() string {
	return os.Getenv("TEST_SCYLLADB_CLOUD_EAR_KEY_ID")
}

func getClientFromProvider(provider *schema.Provider) *scylla.Client {
	return provider.Meta().(*scylla.Client)
}

func parseClusterIDFromResourceID(resourceID string) (int64, error) {
	if resourceID == "" {
		return 0, errors.Errorf("cluster ID not set")
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid cluster ID %q", resourceID)
	}
	return id, nil
}
