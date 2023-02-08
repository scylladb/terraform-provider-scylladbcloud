resource "scylladbcloud_serverless_cluster" "my" {
	name = "My Cluster"
}

output "scylladbcloud_serverless_cluster_id" {
	value = scylladbcloud_serverless_cluster.my.id
}
