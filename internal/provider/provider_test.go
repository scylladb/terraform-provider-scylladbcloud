package provider

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkterraform "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/pkg/errors"

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

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: `resource "scylladbcloud_cluster" "basic-aws" {
  name       = "basic-aws"
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  node_count = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.basic-aws", &cluster),
					resource.TestCheckResourceAttr("scylladbcloud_cluster.basic-aws", "name", "basic-aws"),
				),
			},
			{
				Config: `resource "scylladbcloud_cluster" "basic-aws" {
  name       = "basic-aws"
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  node_count = 6
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterNodeCountUpdate(ctx, "scylladbcloud_cluster.basic-aws", 6, &cluster),
					resource.TestCheckResourceAttr("scylladbcloud_cluster.basic-aws", "name", "basic-aws"),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicGCP(t *testing.T) {
	ctx := t.Context()

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: `resource "scylladbcloud_cluster" "basic-gcp" {
  name       = "basic-gcp"
  cloud      = "GCP"
  region     = "us-central1"
  node_type  = "n2d-highmem-2"
  node_count = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.basic-gcp", &cluster),
					resource.TestCheckResourceAttr("scylladbcloud_cluster.basic-gcp", "name", "basic-gcp"),
				),
			},
		},
	})
}

func TestAccScyllaDBCloudCluster_basicGCPBYOA(t *testing.T) {
	ctx := t.Context()

	var cluster model.Cluster

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV5ProviderFactories: protoV5ProviderFactories,
		CheckDestroy:             testAccCheckScyllaDBCloudClusterDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: `resource "scylladbcloud_cluster" "basic-gcp-byoa" {
  name       = "basic-gcp-byoa"
  cloud      = "GCP"
  region     = "us-central1"
  node_type  = "n2d-highmem-2"
  node_count = 3
  cidr_block = "10.0.1.0/24"
  enable_dns = true
  byoa_id    = "18829"
}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckScyllaDBCloudClusterExists(ctx, "scylladbcloud_cluster.basic-gcp-byoa", &cluster),
					resource.TestCheckResourceAttr("scylladbcloud_cluster.basic-gcp-byoa", "name", "basic-gcp-byoa"),
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

func testAccCheckScyllaDBCloudClusterNodeCountUpdate(
	ctx context.Context,
	resourceName string,
	nodeCount int,
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

		if cluster.ID == response.ID {
			return errors.Errorf("expected cluster ID to change after update, but it did not")
		}

		*cluster = *response

		if len(cluster.Nodes) != nodeCount {
			return errors.Errorf("expected node count to be %d, got %d", nodeCount, len(cluster.Nodes))
		}

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
