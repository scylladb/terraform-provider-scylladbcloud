# A cluster with a fixed node type. Variables left unset are omitted from the
# configuration, so the provider's defaults apply.

variable "name" {
  type = string
}

variable "cloud" {
  type    = string
  default = "AWS"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "node_type" {
  type    = string
  default = "i3.large"
}

variable "min_nodes" {
  type    = number
  default = 3
}

variable "scylla_version" {
  type    = string
  default = null
}

variable "availability_zone_ids" {
  type    = list(string)
  default = null
}

variable "backup_retention_days" {
  type    = number
  default = null
}

variable "byoa_id" {
  type    = number
  default = null
}

# Whether the encryption_at_rest block is written at all.
variable "encryption_at_rest" {
  type    = bool
  default = true
}

variable "encryption_at_rest_enabled" {
  type    = bool
  default = null
}

variable "encryption_at_rest_key_id" {
  type    = string
  default = null
}

resource "scylladbcloud_cluster" "test" {
  name                  = var.name
  cloud                 = var.cloud
  region                = var.region
  node_type             = var.node_type
  min_nodes             = var.min_nodes
  scylla_version        = var.scylla_version
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = var.backup_retention_days
  availability_zone_ids = var.availability_zone_ids
  byoa_id               = var.byoa_id

  dynamic "encryption_at_rest" {
    for_each = var.encryption_at_rest ? [1] : []
    content {
      enabled = var.encryption_at_rest_enabled
      key_id  = var.encryption_at_rest_key_id
    }
  }
}
