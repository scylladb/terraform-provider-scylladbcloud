package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// testAccCheckClusterReplicationFactor checks the replication factor that the
// API reports, not only the value in state.
func testAccCheckClusterReplicationFactor(cluster *model.Cluster, want int64) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if cluster.ReplicationFactor != want {
			return fmt.Errorf("expected replication factor %d from the API, got %d", want, cluster.ReplicationFactor)
		}
		return nil
	}
}

func TestAccScyllaDBCloudCluster_replicationFactorAWS(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("rf-aws")

	var cluster model.Cluster

	clusterIDCompare := statecheck.CompareValue(compare.ValuesSame())

	config := func(minNodes int) string {
		return fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = %[2]d
  replication_factor    = 2
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0

  encryption_at_rest {
    enabled = false
  }
}`, resourceName, minNodes)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				// 4 is not a multiple of 3, so the plan checks min_nodes
				// against the configured replication factor. ScyllaDB Cloud
				// creates no Standard cluster with fewer than 3 nodes.
				Config: config(4),
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("cluster_id"),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("replication_factor"),
						knownvalue.Int64Exact(2),
					),
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("node_count"),
						knownvalue.Int64Exact(4),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
					testAccCheckClusterReplicationFactor(&cluster, 2),
				),
			},
			{
				// A scale-out keeps the same number of nodes in each rack.
				Config: config(6),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
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
						tfjsonpath.New("node_count"),
						knownvalue.Int64Exact(6),
					),
				},
			},
			{
				Config:      config(5),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`min_nodes must be a multiple of replication_factor \(2\), got 5`),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_replicationFactorXCloudAWS(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("rf-xcloud-aws")

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
  replication_factor    = 2

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

  encryption_at_rest {
    enabled = false
  }
}`, resourceName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("replication_factor"),
						knownvalue.Int64Exact(2),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
					testAccCheckClusterReplicationFactor(&cluster, 2),
				),
			},
		},
	})
}

// TestAccScyllaDBCloudCluster_replicationFactorUpgrade checks that adding
// replication_factor is backward compatible. 1.13 has no such attribute, so
// its state is the state every current user has.
//
// Note: comment out any dev_overrides for scylladb/scylladbcloud in
// ~/.terraformrc before running this. An override wins over the version pin
// below and would silently test the local build against itself.
func TestAccScyllaDBCloudCluster_replicationFactorUpgrade(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix("rf-upgrade")

	var cluster model.Cluster

	clusterIDCompare := statecheck.CompareValue(compare.ValuesSame())

	config := func(replicationFactor string) string {
		return fmt.Sprintf(`resource "scylladbcloud_cluster" "test" {
  name                  = %[1]q
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
  %[2]s

  encryption_at_rest {
    enabled = false
  }
}`, resourceName, replicationFactor)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"scylladbcloud": {
						// The last version released without replication_factor.
						VersionConstraint: "1.13",
						Source:            "scylladb/scylladbcloud",
					},
				},
				Config: config(""),
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("id"),
					),
				},
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				Config:                   config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("id"),
					),
					// The refresh writes the value that 1.13 always sent.
					statecheck.ExpectKnownValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("replication_factor"),
						knownvalue.Int64Exact(3),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.test", &cluster),
					testAccCheckClusterReplicationFactor(&cluster, 3),
				),
			},
			{
				// Writing down the real value is not a change.
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				Config:                   config("replication_factor = 3"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					clusterIDCompare.AddStateValue(
						"scylladbcloud_cluster.test",
						tfjsonpath.New("id"),
					),
				},
			},
			{
				// The replication factor is create-time only. PlanOnly keeps
				// this check from costing a second real cluster. An RF of 1
				// keeps min_nodes = 3 valid, so the plan reaches the
				// replacement.
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				Config:                   config("replication_factor = 1"),
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
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
