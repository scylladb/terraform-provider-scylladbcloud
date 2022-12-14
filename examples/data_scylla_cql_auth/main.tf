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

data "scylla_cql_auth" "scylla" {
	cluster_id = 10
}

output "scylla_cql_seeds" {
	value = data.scylla_cql_auth.scylla.seeds
}

output "scylla_cql_username" {
	value = data.scylla_cql_auth.scylla.username
}

output "scylla_cql_password" {
	value = data.scylla_cql_auth.scylla.password
}
