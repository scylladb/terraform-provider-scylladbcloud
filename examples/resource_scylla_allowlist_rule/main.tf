terraform {
	required_providers {
		scylla = {
			source = "registry.terraform.io/scylladb/scylla"
		}
	}
}

provider "scylla" {
	token = "..."
}

resource "scylla_allowlist_rule" "home" {
	cluster_id = 10
	cidr_block = "89.74.148.54/32"
}

output "scylla_allowlist_rule_id" {
	value = scylla_allowlist_rule.home.rule_id
}
