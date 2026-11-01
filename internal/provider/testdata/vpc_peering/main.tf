# A cluster peered with a VPC in another AWS account.

variable "name" {
  type = string
}

variable "cidr_block" {
  type = string
}

variable "peer_vpc_id" {
  type = string
}

variable "peer_cidr_block" {
  type = string
}

variable "peer_account_id" {
  type = string
}

resource "scylladbcloud_cluster" "test" {
  name                  = var.name
  cloud                 = "AWS"
  region                = "us-east-1"
  node_type             = "i3.large"
  min_nodes             = 3
  cidr_block            = var.cidr_block
  enable_dns            = true
  backup_retention_days = 0

  encryption_at_rest {
    enabled = false
  }
}

resource "scylladbcloud_vpc_peering" "test" {
  cluster_id       = scylladbcloud_cluster.test.cluster_id
  datacenter       = scylladbcloud_cluster.test.datacenter
  peer_vpc_id      = var.peer_vpc_id
  peer_cidr_blocks = [var.peer_cidr_block]
  peer_region      = "us-east-1"
  peer_account_id  = var.peer_account_id
  allow_cql        = true
}
