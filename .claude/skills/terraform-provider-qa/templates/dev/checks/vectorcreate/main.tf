terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
  required_providers {
    scylladbcloud = {
      source = "registry.terraform.io/scylladb/scylladbcloud"
    }
  }
}

provider "scylladbcloud" {}

# 0 drops the vector_search block.
variable "vs_nodes" {
  type    = number
  default = 1
}

resource "scylladbcloud_cluster" "vs" {
  name               = "tf-dev-vs"
  cloud              = "AWS"
  region             = "us-east-1"
  min_nodes          = 3
  node_type          = "i8g.large"
  cidr_block         = "172.31.0.0/16"
  enable_vpc_peering = false
  enable_dns         = true

  encryption_at_rest {
    enabled = false
  }

  dynamic "vector_search" {
    for_each = var.vs_nodes > 0 ? [1] : []
    content {
      node_count = var.vs_nodes
      node_type  = "t4g.medium"
    }
  }
}

output "cluster_id" {
  value = scylladbcloud_cluster.vs.cluster_id
}
