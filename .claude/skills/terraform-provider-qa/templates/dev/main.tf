terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
  required_providers {
    scylladbcloud = {
      source  = "registry.terraform.io/scylladb/scylladbcloud"
      version = "= 1.13.0"
    }
  }
}

provider "scylladbcloud" {}

variable "cluster_name" {
  type    = string
  default = "tf-dev"
}

resource "scylladbcloud_cluster" "dev" {
  name               = var.cluster_name
  cloud              = "AWS"
  region             = "us-east-1"
  min_nodes          = 3
  node_type          = "i8g.large"
  cidr_block         = "172.31.0.0/16"
  enable_vpc_peering = false
  enable_dns         = true

  # The local fake BYOK service cannot serve Scylla's KMS config, so
  # encryption at rest is gated in local runs.
  encryption_at_rest {
    enabled = false
  }
}

resource "scylladbcloud_allowlist_rule" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
  cidr_block = "192.0.2.10/32"
}

output "cluster_id" {
  value = scylladbcloud_cluster.dev.cluster_id
}

output "datacenter" {
  value = scylladbcloud_cluster.dev.datacenter
}
