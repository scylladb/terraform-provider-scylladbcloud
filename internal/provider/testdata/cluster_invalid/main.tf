# A cluster configuration that tests make invalid one setting at a time. It is
# only ever planned, so nothing is created. Variables left unset are omitted.

variable "node_type" {
  type    = string
  default = null
}

variable "min_nodes" {
  type    = number
  default = null
}

variable "backup_retention_days" {
  type    = number
  default = null
}

# Whether the scaling block is written at all.
variable "scaling" {
  type    = bool
  default = false
}

variable "instance_families" {
  type    = list(string)
  default = null
}

variable "instance_types" {
  type    = list(string)
  default = null
}

variable "target_utilization" {
  type    = number
  default = 0.75
}

# Whether the encryption_at_rest block is written at all.
variable "encryption_at_rest" {
  type    = bool
  default = false
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
  name                  = "tf-acc-test-invalid"
  cloud                 = "AWS"
  region                = "us-east-1"
  cidr_block            = "10.0.250.0/24"
  node_type             = var.node_type
  min_nodes             = var.min_nodes
  backup_retention_days = var.backup_retention_days

  dynamic "scaling" {
    for_each = var.scaling ? [1] : []
    content {
      instance_families = var.instance_families
      instance_types    = var.instance_types
      storage_policy {
        min_gb             = 500
        target_utilization = var.target_utilization
      }
    }
  }

  dynamic "encryption_at_rest" {
    for_each = var.encryption_at_rest ? [1] : []
    content {
      enabled = var.encryption_at_rest_enabled
      key_id  = var.encryption_at_rest_key_id
    }
  }
}
