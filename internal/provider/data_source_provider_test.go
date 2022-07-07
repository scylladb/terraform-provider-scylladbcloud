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
					resource.TestCheckResourceAttr("data.scyllacloud_provider.test", "id", "1"),
				),
			},
		},
	})
}

// TODO URL used here won't work in the future
const testAccProviderDataSourceConfig = `
terraform {
  required_providers {
    scyllacloud = {
      source  = "registry.terraform.io/scylladb/scyllacloud"
    }
  }
}

provider "scyllacloud" {
	endpoint = "https://api-amnonh.ext.lab.scylla.cloud/api/v0"
	token    = "token"
}

data "scyllacloud_provider" "test" {
  vendor = "AWS"
}
`
