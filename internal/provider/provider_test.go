package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	acc "github.com/scylladb/terraform-provider-scylladbcloud/internal/acctest"
	providercluster "github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	v2scylla "github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/v2"
)

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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, config.Variables{
					"scylla_version":        config.StringVariable("2026.1.1"),
					"availability_zone_ids": config.ListVariable(config.StringVariable("use1-az2"), config.StringVariable("use1-az4"), config.StringVariable("use1-az6")),
				}),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("testdata/cluster_scaling"),
				ConfigVariables: config.Variables{"name": config.StringVariable(resourceName)},
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, config.Variables{
					"availability_zone_ids": config.ListVariable(config.StringVariable("use1-az2")),
				}),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, nil),
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
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, config.Variables{"min_nodes": config.IntegerVariable(6)}),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, nil),
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
					client := acc.Client(t)

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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, testAccGCPVariables),
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

	if envGCPBYOAID() == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID must be set for this test")
	}
	byoaID, err := strconv.Atoi(envGCPBYOAID())
	if err != nil {
		t.Fatalf("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID must be a number: %s", err)
	}

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, testAccGCPVariables, config.Variables{"byoa_id": config.IntegerVariable(byoaID)}),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccClusterVariables(resourceName, config.Variables{"backup_retention_days": nil}),
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

// testAccClusterDir holds a cluster configuration whose variables cover every
// cluster the acceptance tests create with a fixed node type.
var testAccClusterDir = config.StaticDirectory("testdata/cluster")

var testAccGCPVariables = config.Variables{
	"cloud":     config.StringVariable("GCP"),
	"region":    config.StringVariable("us-central1"),
	"node_type": config.StringVariable("n2d-highmem-2"),
}

// testAccClusterVariables returns the variables of testAccClusterDir for a
// three-node, unencrypted AWS cluster that keeps no backups, with overrides
// applied in order. A nil override leaves that variable unset, so the
// provider's default applies.
func testAccClusterVariables(name string, overrides ...config.Variables) config.Variables {
	v := config.Variables{
		"name":                       config.StringVariable(name),
		"backup_retention_days":      config.IntegerVariable(0),
		"encryption_at_rest_enabled": config.BoolVariable(false),
	}
	for _, o := range overrides {
		for k, val := range o {
			if val == nil {
				delete(v, k)
			} else {
				v[k] = val
			}
		}
	}
	return v
}

// testAccEncryptionAtRestVariables returns cluster variables with the given
// encryption at rest settings.
func testAccEncryptionAtRestVariables(name, cloud, region, nodeType string, ear config.Variables) config.Variables {
	return testAccClusterVariables(name, config.Variables{
		"cloud":     config.StringVariable(cloud),
		"region":    config.StringVariable(region),
		"node_type": config.StringVariable(nodeType),
	}, ear)
}

func earEnabled(enabled bool) config.Variables {
	return config.Variables{"encryption_at_rest_enabled": config.BoolVariable(enabled)}
}

// earKey sets a customer-managed key and leaves "enabled" out on purpose, so
// the test also covers its default.
func earKey(keyID string) config.Variables {
	return config.Variables{"encryption_at_rest_enabled": nil, "encryption_at_rest_key_id": config.StringVariable(keyID)}
}

// earAbsent leaves the encryption_at_rest block out of the configuration.
var earAbsent = config.Variables{"encryption_at_rest": config.BoolVariable(false), "encryption_at_rest_enabled": nil}

func TestAccScyllaDBCloudCluster_encryptionAtRestEnabled(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-enabled")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", "i3.large", earEnabled(true)),
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
				ConfigDirectory:    testAccClusterDir,
				ConfigVariables:    testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", "i3.large", earEnabled(false)),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", testAccAWSNodeType("i3.large"), earEnabled(false)),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "GCP", "us-central1", "n2d-highmem-2", earEnabled(true)),
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
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", "i3.large", earKey(keyID)),
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

func TestAccScyllaDBCloudCluster_encryptionAtRestDefault(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-default")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", "i3.large", earAbsent),
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
				// The block is Computed, so leaving it out of the configuration
				// must keep planning a no-op rather than a replacement.
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", "i3.large", earAbsent),
				PlanOnly:        true,
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_encryptionAtRestDefaultGCP(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-default-gcp")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccEncryptionAtRestVariables(resourceName, "GCP", "us-central1", "n2d-highmem-2", earAbsent),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("enabled"),
						knownvalue.Bool(true),
					),
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

// TestAccScyllaDBCloudCluster_encryptionAtRestNoDiffOnUpgrade verifies that
// making encryption-at-rest default is backward compatible.
//
// encryption_at_rest is ForceNew, so if an absent block ever diffed against an
// existing cluster, upgrading the provider would destroy and recreate every
// cluster under management. 1.12 has no encryption_at_rest attribute at all,
// which makes it the real upgrade path: state written before the attribute
// existed, read by a provider that now defaults it on.
//
// Note: comment out any dev_overrides for scylladb/scylladbcloud in
// ~/.terraformrc before running this. An override wins over the version pin
// below and would silently test the local build against itself.
func TestAccScyllaDBCloudCluster_encryptionAtRestNoDiffOnUpgrade(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("ear-no-diff-on-upgrade")

	var cluster model.Cluster

	clusterIDCompare := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acc.PreCheck(t) },
		CheckDestroy: testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				// testdata/cluster_v1_12 pins 1.12, the last version released
				// without encryption at rest.
				ConfigDirectory: config.StaticDirectory("testdata/cluster_v1_12"),
				ConfigVariables: config.Variables{"name": config.StringVariable(resourceName)},
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("id"),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				ConfigDirectory:          testAccClusterDir,
				ConfigVariables:          testAccEncryptionAtRestVariables(resourceName, "AWS", "us-east-1", "i3.large", earAbsent),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Same cluster, not a replacement.
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("id"),
					),
					// The cluster predates the default and stays unencrypted.
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("enabled"),
						knownvalue.Bool(false),
					),
				},
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
		PreCheck:     func() { acc.PreCheck(t) },
		CheckDestroy: testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				// testdata/cluster_v1 pins 1.9, the last version with the v1
				// schema (node_count instead of min_nodes).
				ConfigDirectory: config.StaticDirectory("testdata/cluster_v1"),
				ConfigVariables: config.Variables{"name": config.StringVariable(resourceName)},
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
				ConfigDirectory:          testAccClusterDir,
				ConfigVariables:          testAccClusterVariables(resourceName, nil),
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

func testAccAWSNodeType(def string) string {
	return nonempty(os.Getenv("SCYLLADB_CLOUD_TEST_AWS_NODE_TYPE"), def)
}

func testAccCheckScyllaDBCloudClusterExists(
	ctx context.Context,
	resourceName string,
	cluster *model.Cluster,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := acc.NewClient()
		if err != nil {
			return err
		}

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
		client, err := acc.NewClient()
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "scylladbcloud_cluster" {
				continue
			}

			clusterID, err := parseClusterIDFromResourceID(rs.Primary.ID)
			if err != nil {
				return err
			}

			if err := clusterDestroyed(ctx, client, clusterID); err != nil {
				return err
			}
		}

		return nil
	}
}

// clusterDestroyed returns nil only when the API confirms the cluster is gone;
// any other error is returned so an expired token or a 5xx cannot pass for a
// destroyed cluster.
func clusterDestroyed(ctx context.Context, c *scylla.Client, clusterID int64) error {
	_, err := c.GetClusterSummary(ctx, clusterID)
	switch {
	case err == nil:
		return errors.Errorf("cluster %d still exists", clusterID)
	case scylla.IsNotFound(err), scylla.IsDeletedErr(err), scylla.IsClusterDeletedErr(err):
		return nil
	}
	return errors.Wrapf(err, "checking that cluster %d was destroyed", clusterID)
}

func envGCPBYOAID() string {
	return os.Getenv("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID")
}

func envEARKeyID() string {
	return os.Getenv("TEST_SCYLLADB_CLOUD_EAR_KEY_ID")
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
