terraform {
	required_providers {
		scylladbclouddbcloud = {
			source = "registry.terraform.io/scylladb/scylladbclouddbcloud"
		}
	}
}

provider "scylladbcloud" {
	token = "..."
}

resource "scylladbcloud_allowlist_rule" "home" {
	cluster_id = 10
	cidr_block = "89.74.148.54/32"
}

output "scylladbcloud_allowlist_rule_id" {
	value = scylladbcloud_allowlist_rule.home.rule_id
}
