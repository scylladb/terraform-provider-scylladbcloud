package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/acctest"
	providercluster "github.com/scylladb/terraform-provider-scylladbcloud/internal/provider/cluster"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
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

// The configurations of the acceptance tests are in testdata. testAccClusterDir
// is the cluster that most tests use. Its variables come from
// testAccAWSClusterVars or testAccGCPClusterVars.
var (
	testAccClusterDir            = config.StaticDirectory("testdata/cluster")
	testAccClusterXCloudDir      = config.StaticDirectory("testdata/cluster_xcloud")
	testAccClusterProvider112Dir = config.StaticDirectory("testdata/cluster_provider_1_12")
	testAccClusterProvider19Dir  = config.StaticDirectory("testdata/cluster_provider_1_9")
)

// testAccAWSClusterVars returns the variables of testAccClusterDir for the
// usual AWS cluster: 3 i3.large nodes, no backups and no encryption at rest.
// A test changes a copy to set other values.
func testAccAWSClusterVars(name string) config.Variables {
	return config.Variables{
		"name":                  config.StringVariable(name),
		"cloud":                 config.StringVariable("AWS"),
		"region":                config.StringVariable("us-east-1"),
		"node_type":             config.StringVariable("i3.large"),
		"min_nodes":             config.IntegerVariable(3),
		"backup_retention_days": config.IntegerVariable(0),
		"encryption_at_rest":    testAccEncryptionEnabled(false),
	}
}

// testAccGCPClusterVars is testAccAWSClusterVars for GCP.
func testAccGCPClusterVars(name string) config.Variables {
	v := testAccAWSClusterVars(name)
	v["cloud"] = config.StringVariable("GCP")
	v["region"] = config.StringVariable("us-central1")
	v["node_type"] = config.StringVariable("n2d-highmem-2")
	return v
}

// testAccEncryptionEnabled is an encryption_at_rest block with only enabled.
func testAccEncryptionEnabled(enabled bool) config.Variable {
	return config.ObjectVariable(map[string]config.Variable{"enabled": config.BoolVariable(enabled)})
}

// testAccEncryptionKey is an encryption_at_rest block with only key_id, so
// that the test also covers the default of enabled.
func testAccEncryptionKey(keyID string) config.Variable {
	return config.ObjectVariable(map[string]config.Variable{"key_id": config.StringVariable(keyID)})
}

// TestAccScyllaDBCloudCluster_basicAWS is in the smoke set, so it checks as
// much as it can on its one cluster: an allowlist rule, the cql_auth data
// source, and the import of the cluster and the rule. The framework checks
// that the plan after the apply is empty. With the variables of acctest.AWSPeerVPC set, it also creates a
// VPC peering.
func TestAccScyllaDBCloudCluster_basicAWS(t *testing.T) {
	resourceName := sdkacctest.RandomWithPrefix("basic-aws")
	vars := testAccAWSClusterVars(resourceName)
	vars["scylla_version"] = config.StringVariable("2026.1.1")
	vars["availability_zone_ids"] = config.SetVariable(
		config.StringVariable("use1-az2"),
		config.StringVariable("use1-az4"),
		config.StringVariable("use1-az6"),
	)
	vars["allowlist_cidr"] = config.StringVariable("192.0.2.0/24")
	vars["read_cql_auth"] = config.BoolVariable(true)

	var peerChecks []statecheck.StateCheck
	if vpcID, accountID, cidr, ok := acctest.AWSPeerVPC(); ok {
		vars["peer"] = config.ObjectVariable(map[string]config.Variable{
			"vpc_id":     config.StringVariable(vpcID),
			"account_id": config.StringVariable(accountID),
			"cidr":       config.StringVariable(cidr),
		})
		peerChecks = []statecheck.StateCheck{
			statecheck.ExpectKnownValue(
				"scylladbcloud_vpc_peering.test[0]",
				tfjsonpath.New("peer_vpc_id"),
				knownvalue.StringExact(vpcID),
			),
			statecheck.ExpectKnownValue(
				"scylladbcloud_vpc_peering.test[0]",
				tfjsonpath.New("connection_id"),
				knownvalue.StringRegexp(regexp.MustCompile(`^pcx-`)),
			),
			statecheck.CompareValuePairs(
				"scylladbcloud_vpc_peering.test[0]", tfjsonpath.New("cluster_id"),
				"scylladbcloud_cluster.test", tfjsonpath.New("cluster_id"),
				compare.ValuesSame(),
			),
		}
	}

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
				ConfigStateChecks: append([]statecheck.StateCheck{
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
					// The configuration turns encryption at rest off.
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
					statecheck.CompareValuePairs(
						"scylladbcloud_allowlist_rule.test[0]", tfjsonpath.New("cluster_id"),
						"scylladbcloud_cluster.test", tfjsonpath.New("cluster_id"),
						compare.ValuesSame(),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_allowlist_rule.test[0]",
						tfjsonpath.New("cidr_block"),
						knownvalue.StringExact("192.0.2.0/24"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_allowlist_rule.test[0]",
						tfjsonpath.New("rule_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.scylladbcloud_cql_auth.test[0]",
						tfjsonpath.New("username"),
						knownvalue.StringRegexp(regexp.MustCompile(`.`)),
					),
					statecheck.ExpectKnownValue(
						"data.scylladbcloud_cql_auth.test[0]",
						tfjsonpath.New("password"),
						knownvalue.StringRegexp(regexp.MustCompile(`.`)),
					),
					statecheck.ExpectKnownValue(
						"data.scylladbcloud_cql_auth.test[0]",
						tfjsonpath.New("seeds"),
						knownvalue.StringRegexp(regexp.MustCompile(`.`)),
					),
				}, peerChecks...),
				Check: resource.ComposeTestCheckFunc(
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
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
			{
				ConfigDirectory:   testAccClusterDir,
				ConfigVariables:   vars,
				ResourceName:      "scylladbcloud_cluster.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Read never sets backup_retention_days, and sets
				// alternator_write_isolation only for Alternator clusters.
				ImportStateVerifyIgnore: []string{"backup_retention_days", "alternator_write_isolation"},
			},
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
				// The address has the count index. The state has the
				// resource at "test.0", so the ID must come from there.
				ResourceName:      "scylladbcloud_allowlist_rule.test[0]",
				ImportStateIdFunc: testAccResourceID("scylladbcloud_allowlist_rule.test.0"),
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// testAccResourceID returns the ID of the resource at key in the state.
func testAccResourceID(key string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[key]
		if !ok || rs.Primary == nil {
			return "", fmt.Errorf("resource %q not found in state", key)
		}
		return rs.Primary.ID, nil
	}
}

func TestAccScyllaDBCloudCluster_xcloudAWS(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("xcloud-aws")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterXCloudDir,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicAWSSingleAZ(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("basic-aws-single-az")
	vars := testAccAWSClusterVars(resourceName)
	vars["availability_zone_ids"] = config.SetVariable(config.StringVariable("use1-az2"))

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicAWSScaleOut(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("basic-aws-scale-out")
	scaledOut := testAccAWSClusterVars(resourceName)
	scaledOut["min_nodes"] = config.IntegerVariable(6)

	var cluster model.Cluster

	clusterIDCompare := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccAWSClusterVars(resourceName),
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: scaledOut,
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
	acctest.SkipIfShort(t)
	ctx := t.Context()
	resourceName := sdkacctest.RandomWithPrefix("basic-aws-scale-out-outside")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccAWSClusterVars(resourceName),
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				PreConfig: func() {
					client := acctest.Client(t)

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
	resourceName := sdkacctest.RandomWithPrefix("basic-gcp")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: testAccGCPClusterVars(resourceName),
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicGCPBYOA(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("basic-gcp-byoa")

	byoaID := acctest.GCPBYOAID()
	if byoaID == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID must be set for this test")
	}
	id, err := strconv.Atoi(byoaID)
	if err != nil {
		t.Fatalf("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID must be a number: %v", err)
	}
	vars := testAccGCPClusterVars(resourceName)
	vars["byoa_id"] = config.IntegerVariable(id)

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_encryptionAtRestEnabled(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("ear-enabled")
	enabled := testAccAWSClusterVars(resourceName)
	enabled["encryption_at_rest"] = testAccEncryptionEnabled(true)
	disabled := testAccAWSClusterVars(resourceName)

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: enabled,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				// Encryption at rest is create-time only, so any change must
				// plan a replacement rather than an in-place update. PlanOnly
				// keeps this assertion from costing a second real cluster.
				ConfigDirectory:    testAccClusterDir,
				ConfigVariables:    disabled,
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

// TestAccScyllaDBCloudCluster_encryptionAtRestGCP requires the enable_ear_gcp
// feature flag to be on in the target environment.
func TestAccScyllaDBCloudCluster_encryptionAtRestGCP(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("ear-gcp")
	vars := testAccGCPClusterVars(resourceName)
	vars["encryption_at_rest"] = testAccEncryptionEnabled(true)

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("encryption_at_rest").AtSliceIndex(0).AtMapKey("provider"),
						knownvalue.StringExact("scylla-gcp"),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

// TestAccScyllaDBCloudCluster_encryptionAtRestCMK needs a customer-managed key
// created up front in the ScyllaDB Cloud portal; there is no public keys API.
func TestAccScyllaDBCloudCluster_encryptionAtRestCMK(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("ear-cmk")

	keyID := acctest.EARKeyID()
	if keyID == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_EAR_KEY_ID must be set for this test")
	}
	vars := testAccAWSClusterVars(resourceName)
	vars["encryption_at_rest"] = testAccEncryptionKey(keyID)

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
		},
	})
}

// TestAccScyllaDBCloudCluster_defaultsAWS checks the defaults of an AWS
// cluster on one cluster: the configuration sets neither
// backup_retention_days nor encryption_at_rest.
func TestAccScyllaDBCloudCluster_defaultsAWS(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("defaults-aws")
	vars := testAccAWSClusterVars(resourceName)
	delete(vars, "backup_retention_days")
	delete(vars, "encryption_at_rest")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("backup_retention_days"),
						knownvalue.Int32Exact(1),
					),
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				// The block is Computed, so leaving it out of the configuration
				// must keep planning a no-op rather than a replacement.
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
				PlanOnly:        true,
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_encryptionAtRestDefaultGCP(t *testing.T) {
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("ear-default-gcp")
	vars := testAccGCPClusterVars(resourceName)
	delete(vars, "encryption_at_rest")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: testAccClusterDir,
				ConfigVariables: vars,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
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
	acctest.SkipIfShort(t)
	resourceName := sdkacctest.RandomWithPrefix("ear-no-diff-on-upgrade")

	var cluster model.Cluster

	clusterIDCompare := statecheck.CompareValue(compare.ValuesSame())

	// The same cluster as testAccClusterProvider112Dir, for the local build.
	vars := testAccAWSClusterVars(resourceName)
	delete(vars, "encryption_at_rest")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		CheckDestroy: acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				// The configuration pins provider 1.12.
				ConfigDirectory: testAccClusterProvider112Dir,
				ConfigVariables: config.Variables{"name": config.StringVariable(resourceName)},
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("id"),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				ConfigDirectory:          testAccClusterDir,
				ConfigVariables:          vars,
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
	resourceName := sdkacctest.RandomWithPrefix("basic-aws-migration-v1-to-v2")

	var cluster model.Cluster

	nodeCountCompare := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		CheckDestroy: acctest.CheckClusterDestroyed(t),
		Steps: []resource.TestStep{
			{
				// The configuration pins provider 1.9, the last release with
				// the v1 schema.
				ConfigDirectory: testAccClusterProvider19Dir,
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
					acctest.CheckClusterExists(t, "scylladbcloud_cluster.test", &cluster),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				ConfigDirectory:          testAccClusterDir,
				ConfigVariables:          testAccAWSClusterVars(resourceName),
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
