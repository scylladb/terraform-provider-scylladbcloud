package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.scylla_provider.test", "id", "1"),
				),
			},
		},
	})
}

// TODO URL used here won't work in the future
const testAccProviderDataSourceConfig = `
terraform {
  required_providers {
    scylla = {
      source  = "registry.terraform.io/scylladb/scylla"
    }
  }
}

provider "scylla" {
	endpoint = "https://api-amnonh.ext.lab.scylla.cloud/api/v0"
	token    = "token"
}

data "scylla_provider" "test" {
  vendor = "AWS"
}
`
