# Optional resources on the cluster of main.tf. A test turns them on with
# variables, so that it can check them without a cluster of its own. Each
# cluster costs about 25 minutes. By default, none of them exists.

variable "allowlist_cidr" {
  type    = string
  default = null
}

variable "read_cql_auth" {
  type    = bool
  default = false
}

# A VPC in the region of the cluster, in the account of account_id.
variable "peer" {
  type = object({
    vpc_id     = string
    account_id = string
    cidr       = string
  })
  default = null
}

resource "scylladbcloud_allowlist_rule" "test" {
  count = var.allowlist_cidr == null ? 0 : 1

  cluster_id = scylladbcloud_cluster.test.cluster_id
  cidr_block = var.allowlist_cidr
}

data "scylladbcloud_cql_auth" "test" {
  count = var.read_cql_auth ? 1 : 0

  cluster_id = scylladbcloud_cluster.test.cluster_id
  datacenter = scylladbcloud_cluster.test.datacenter
}

resource "scylladbcloud_vpc_peering" "test" {
  count = var.peer == null ? 0 : 1

  cluster_id       = scylladbcloud_cluster.test.cluster_id
  datacenter       = scylladbcloud_cluster.test.datacenter
  peer_vpc_id      = var.peer.vpc_id
  peer_cidr_blocks = [var.peer.cidr]
  peer_region      = var.region
  peer_account_id  = var.peer.account_id
  allow_cql        = true
}
