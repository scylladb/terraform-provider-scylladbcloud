package provider

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

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

func TestAccScyllaDBCloudCluster_basicAWSScaleOut(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws-scale-out")

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
  name       = %[1]q
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  min_nodes  = 6
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`, resourceName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction("scylladbcloud_cluster.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValue(compare.ValuesSame()).AddStateValue(
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

					req, err := client.ResizeCluster(ctx, cluster.ID, cluster.Datacenter.ID, cluster.InstanceID, 6)
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

func TestAccScyllaDBCloudCluster_basicAWSMigrationV1ToV2(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-aws-migration-v1-to-v2")

	var cluster model.Cluster

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
  name       = %[1]q
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  min_nodes  = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`, resourceName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValue(compare.ValuesSame()).AddStateValue(
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

func TestAccScyllaDBCloudCluster_basicGCPBYOA(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("basic-gcp-byoa")

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name       = %[1]q
  cloud      = "GCP"
  region     = "us-central1"
  node_type  = "n2d-highmem-2"
  min_nodes  = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
  byoa_id    = "18829" // TODO: make configurable via env var
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

		response, err := client.GetCluster(ctx, clusterID)
		if err != nil {
			return errors.Wrapf(err, "error retrieving cluster %d", clusterID)
		}

		*cluster = *response

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

func getClientFromProvider(provider *schema.Provider) *scylla.Client {
	return provider.Meta().(*scylla.Client)
}
