# A cluster attached to an AWS transit gateway.

variable "name" {
  type = string
}

variable "cidr_block" {
  type = string
}

variable "tgw_id" {
  type = string
}

variable "ram_arn" {
  type = string
}

variable "routed_cidr" {
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

resource "scylladbcloud_cluster_connection" "test" {
  cluster_id = scylladbcloud_cluster.test.cluster_id
  name       = var.name
  datacenter = scylladbcloud_cluster.test.datacenter
  cidrlist   = [var.routed_cidr]
  type       = "AWS_TGW_ATTACHMENT"
  data = {
    tgwid  = var.tgw_id
    ramarn = var.ram_arn
  }
}
