# A cluster with an allowlist rule, read back through the cql_auth data source.

variable "name" {
  type = string
}

variable "cidr_block" {
  type = string
}

variable "node_type" {
  type    = string
  default = "i3.large"
}

variable "allowed_cidr" {
  type = string
}

resource "scylladbcloud_cluster" "test" {
  name                  = var.name
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = var.node_type
  min_nodes             = 3
  cidr_block            = var.cidr_block
  enable_vpc_peering    = false
  enable_dns            = true
  backup_retention_days = 0

  encryption_at_rest {
    enabled = false
  }
}

resource "scylladbcloud_allowlist_rule" "test" {
  cluster_id = scylladbcloud_cluster.test.cluster_id
  cidr_block = var.allowed_cidr
}

data "scylladbcloud_cql_auth" "test" {
  cluster_id = scylladbcloud_cluster.test.cluster_id
}
