terraform {
  required_providers {
    scylladbcloud = {
      source = "registry.terraform.io/scylladb/scylladbcloud"
    }
  }
}

# Configuration-based authentication.
provider "scylladbcloud" {
  token    = var.lab_tkn
  endpoint = var.api_endpoint
}

