data "scylladbcloud_cql_auth" "mycluster" {
	cluster_id = 1337
	datacenter = "AWS_US_EAST_1"
}

output "scylladbcloud_cql_seeds" {
	value = data.scylladbcloud_cql_auth.mycluster.seeds
}

output "scylladbcloud_cql_username" {
	value = data.scylladbcloud_cql_auth.mycluster.username
}

output "scylladbcloud_cql_password" {
    sensitive = true
	value = data.scylladbcloud_cql_auth.mycluster.password
}
