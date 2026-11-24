# The cluster for most acceptance tests. Each test sets the variables that it
# needs. A variable that is null leaves its argument out of the configuration.

variable "name" {
  type = string
}

variable "cloud" {
  type = string
}

variable "region" {
  type = string
}

variable "node_type" {
  type = string
}

variable "min_nodes" {
  type = number
}

variable "backup_retention_days" {
  type    = number
  default = null
}

variable "scylla_version" {
  type    = string
  default = null
}

variable "availability_zone_ids" {
  type    = set(string)
  default = null
}

variable "byoa_id" {
  type    = number
  default = null
}

# null leaves the encryption_at_rest block out of the configuration.
variable "encryption_at_rest" {
  type = object({
    enabled = optional(bool)
    key_id  = optional(string)
  })
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
    for_each = var.encryption_at_rest == null ? [] : [var.encryption_at_rest]
    content {
      enabled = encryption_at_rest.value.enabled
      key_id  = encryption_at_rest.value.key_id
    }
  }
}
