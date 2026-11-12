package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	acc "github.com/scylladb/terraform-provider-scylladbcloud/internal/acctest"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

var nonEmpty = regexp.MustCompile(`.`)

func TestAccScyllaDBCloudAllowlistRule_basic(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix(testAccPrefix + "allowlist")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckAllowlistRuleDestroy(ctx),
			testAccCheckScyllaDBCloudClusterDestroy(ctx),
		),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("testdata/allowlist_rule"),
				ConfigVariables: config.Variables{
					"name":         config.StringVariable(resourceName),
					"cidr_block":   config.StringVariable(testAccCIDR(resourceName)),
					"node_type":    config.StringVariable(testAccAWSNodeType("i3.large")),
					"allowed_cidr": config.StringVariable("192.0.2.20/32"),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("scylladbcloud_allowlist_rule.test", tfjsonpath.New("cidr_block"), knownvalue.StringExact("192.0.2.20/32")),
					statecheck.ExpectKnownValue("scylladbcloud_allowlist_rule.test", tfjsonpath.New("rule_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.scylladbcloud_cql_auth.test", tfjsonpath.New("username"), knownvalue.StringRegexp(nonEmpty)),
					statecheck.ExpectKnownValue("data.scylladbcloud_cql_auth.test", tfjsonpath.New("password"), knownvalue.StringRegexp(nonEmpty)),
					statecheck.ExpectKnownValue("data.scylladbcloud_cql_auth.test", tfjsonpath.New("seeds"), knownvalue.StringRegexp(nonEmpty)),
				},
			},
			{
				ResourceName:      "scylladbcloud_allowlist_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				ConfigDirectory:   config.StaticDirectory("testdata/allowlist_rule"),
				ConfigVariables: config.Variables{
					"name":         config.StringVariable(resourceName),
					"cidr_block":   config.StringVariable(testAccCIDR(resourceName)),
					"node_type":    config.StringVariable(testAccAWSNodeType("i3.large")),
					"allowed_cidr": config.StringVariable("192.0.2.20/32"),
				},
			},
		},
	})
}

// TestAccScyllaDBCloudClusterConnection_basic attaches a cluster to a transit
// gateway shared with the ScyllaDB Cloud account through AWS RAM.
func TestAccScyllaDBCloudClusterConnection_basic(t *testing.T) {
	ctx := t.Context()
	tgwID, ramARN := os.Getenv("TEST_SCYLLADB_CLOUD_TGW_ID"), os.Getenv("TEST_SCYLLADB_CLOUD_TGW_RAM_ARN")
	if tgwID == "" || ramARN == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_TGW_ID and TEST_SCYLLADB_CLOUD_TGW_RAM_ARN must be set for this test")
	}
	resourceName := acctest.RandomWithPrefix(testAccPrefix + "connection")
	vars := config.Variables{
		"name":        config.StringVariable(resourceName),
		"cidr_block":  config.StringVariable(testAccCIDR(resourceName)),
		"tgw_id":      config.StringVariable(tgwID),
		"ram_arn":     config.StringVariable(ramARN),
		"routed_cidr": config.StringVariable("10.201.0.0/16"),
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckClusterConnectionDestroy(ctx),
			testAccCheckScyllaDBCloudClusterDestroy(ctx),
		),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("testdata/cluster_connection"),
				ConfigVariables: vars,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("scylladbcloud_cluster_connection.test", tfjsonpath.New("type"), knownvalue.StringExact("AWS_TGW_ATTACHMENT")),
					statecheck.ExpectKnownValue("scylladbcloud_cluster_connection.test", tfjsonpath.New("external_id"), knownvalue.StringRegexp(nonEmpty)),
				},
			},
			{
				ResourceName:      "scylladbcloud_cluster_connection.test",
				ImportState:       true,
				ImportStateVerify: true,
				ConfigDirectory:   config.StaticDirectory("testdata/cluster_connection"),
				ConfigVariables:   vars,
			},
		},
	})
}

func testAccPeeringVariables(t *testing.T, resourceName string) config.Variables {
	t.Helper()
	vpcID, account, cidr := os.Getenv("TEST_SCYLLADB_CLOUD_PEER_VPC_ID"), os.Getenv("TEST_SCYLLADB_CLOUD_PEER_ACCOUNT_ID"), os.Getenv("TEST_SCYLLADB_CLOUD_PEER_CIDR")
	if vpcID == "" || account == "" || cidr == "" {
		t.Skip("TEST_SCYLLADB_CLOUD_PEER_VPC_ID, TEST_SCYLLADB_CLOUD_PEER_ACCOUNT_ID and TEST_SCYLLADB_CLOUD_PEER_CIDR must be set for this test")
	}
	return config.Variables{
		"name":            config.StringVariable(resourceName),
		"cidr_block":      config.StringVariable(testAccCIDR(resourceName)),
		"peer_vpc_id":     config.StringVariable(vpcID),
		"peer_cidr_block": config.StringVariable(cidr),
		"peer_account_id": config.StringVariable(account),
	}
}

// TestAccScyllaDBCloudVPCPeering_basic peers a cluster with a VPC in another
// AWS account. The peering is not accepted on the AWS side, so it stays
// pending, which is enough to create, read, import and delete it.
func TestAccScyllaDBCloudVPCPeering_basic(t *testing.T) {
	ctx := t.Context()
	resourceName := acctest.RandomWithPrefix(testAccPrefix + "vpc-peering")
	vars := testAccPeeringVariables(t, resourceName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.PreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("testdata/vpc_peering"),
				ConfigVariables: vars,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("scylladbcloud_vpc_peering.test", tfjsonpath.New("connection_id"), knownvalue.StringRegexp(nonEmpty)),
				},
			},
			{
				ResourceName:      "scylladbcloud_vpc_peering.test",
				ImportState:       true,
				ImportStateVerify: true,
				ConfigDirectory:   config.StaticDirectory("testdata/vpc_peering"),
				ConfigVariables:   vars,
			},
		},
	})
}

func testAccCheckAllowlistRuleDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := acc.NewClient()
		if err != nil {
			return err
		}
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "scylladbcloud_allowlist_rule" {
				continue
			}
			clusterID, err := strconv.ParseInt(rs.Primary.Attributes["cluster_id"], 10, 64)
			if err != nil {
				return err
			}
			rules, err := client.ListAllowlistRules(ctx, clusterID)
			switch {
			case scylla.IsNotFound(err), scylla.IsDeletedErr(err), scylla.IsClusterDeletedErr(err):
				continue
			case err != nil:
				return fmt.Errorf("checking that allowlist rule %s was destroyed: %w", rs.Primary.ID, err)
			}
			for _, r := range rules {
				if strconv.FormatInt(r.ID, 10) == rs.Primary.ID {
					return fmt.Errorf("allowlist rule %s still exists", rs.Primary.ID)
				}
			}
		}
		return nil
	}
}

func testAccCheckClusterConnectionDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := acc.NewClient()
		if err != nil {
			return err
		}
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "scylladbcloud_cluster_connection" {
				continue
			}
			clusterID, err := strconv.ParseInt(rs.Primary.Attributes["cluster_id"], 10, 64)
			if err != nil {
				return err
			}
			connID, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil {
				return err
			}
			conn, err := client.GetClusterConnection(ctx, clusterID, connID)
			switch {
			case scylla.IsNotFound(err), scylla.IsDeletedErr(err), scylla.IsClusterDeletedErr(err), scylla.IsClusterConnectionDeletedErr(err):
				continue
			case err != nil:
				return fmt.Errorf("checking that cluster connection %d was destroyed: %w", connID, err)
			case strings.EqualFold(conn.Status, "DELETED"):
				continue
			}
			return fmt.Errorf("cluster connection %d still exists", connID)
		}
		return nil
	}
}
