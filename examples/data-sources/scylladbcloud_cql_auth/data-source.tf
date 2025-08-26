# Fetch credential information for cluster identified by 1337.
data "scylladbcloud_cql_auth" "example" {
	cluster_id = 1337
	datacenter = "AWS_US_EAST_1"
}

output "scylladbcloud_cql_seeds" {
	value = data.scylladbcloud_cql_auth.example.seeds
}

output "scylladbcloud_cql_username" {
	value = data.scylladbcloud_cql_auth.example.username
}

output "scylladbcloud_cql_password" {
    sensitive = true
	value = data.scylladbcloud_cql_auth.example.password
}

output "scylladbcloud_cql_cluster_name" {
  value = data.scylladbcloud_cql_auth.example.cluster_name
}
