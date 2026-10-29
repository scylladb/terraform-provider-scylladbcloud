# A cluster written by provider 1.12, the last release without
# encryption_at_rest. A dev_overrides entry for scylladb/scylladbcloud in
# ~/.terraformrc wins over this pin.

terraform {
  required_providers {
    scylladbcloud = {
      source  = "scylladb/scylladbcloud"
      version = "1.12"
    }
  }
}

# Configured from SCYLLADB_CLOUD_TOKEN and SCYLLADB_CLOUD_ENDPOINT.
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
