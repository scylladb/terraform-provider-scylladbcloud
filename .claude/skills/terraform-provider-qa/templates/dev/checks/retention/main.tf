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

variable "cluster_id" {
  type = number
}

variable "days" {
  type = number
}

resource "scylladbcloud_cluster_backup_retention" "dev" {
  cluster_id     = var.cluster_id
  retention_days = var.days
}
