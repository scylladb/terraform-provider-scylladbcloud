terraform {
	required_providers {
		scylladbcloud = {
			source = "registry.terraform.io/scylladb/scylladbcloud"
		}
	}
}

provider "scylladbcloud" {
	token = "..."
}

data "scylladbcloud_cql_auth" "scylla" {
	cluster_id = 10
}

output "scylladbcloud_cql_seeds" {
	value = data.scylladbcloud_cql_auth.scylla.seeds
}

output "scylladbcloud_cql_username" {
	value = data.scylladbcloud_cql_auth.scylla.username
}

output "scylladbcloud_cql_password" {
	value = data.scylladbcloud_cql_auth.scylla.password
}
