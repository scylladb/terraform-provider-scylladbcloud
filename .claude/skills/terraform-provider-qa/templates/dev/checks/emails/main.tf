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

variable "emails" {
  type = list(string)
}

resource "scylladbcloud_notification_emails" "dev" {
  emails = var.emails
}
