# A cluster sized by its scaling policy instead of a fixed node type.

variable "name" {
  type = string
}

resource "scylladbcloud_cluster" "test" {
  name                  = var.name
  cloud                 = "AWS"
  region                = "us-east-1"
  cidr_block            = "10.0.1.0/24"
  enable_dns            = true
  backup_retention_days = 0
  availability_zone_ids = ["use1-az2", "use1-az4", "use1-az6"]

  scaling {
    instance_families = ["i4i"]
    storage_policy {
      min_gb             = 500
      target_utilization = 0.75
    }
    vcpu_policy {
      min = 8
    }
  }

  encryption_at_rest {
    enabled = false
  }
}
