# The cluster as provider 1.12 creates it. 1.12 is the last release without
# encryption at rest. The upgrade test then plans the same cluster with the
# local build.
#
# A dev_overrides entry for scylladb/scylladbcloud in ~/.terraformrc wins
# over this version pin and silently tests the local build against itself.

terraform {
  required_providers {
    scylladbcloud = {
      source  = "scylladb/scylladbcloud"
      version = "1.12"
    }
  }
}

# The test framework uses the provider of this file only when the file has a
# provider block. The provider reads its settings from the environment.
provider "scylladbcloud" {}

variable "name" {
  type = string
}

resource "scylladbcloud_cluster" "test" {
  name                  = var.name
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
}
