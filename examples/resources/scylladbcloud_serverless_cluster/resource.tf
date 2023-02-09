# Create a serverless cluster.
resource "scylladbcloud_serverless_cluster" "example" {
	name = "My Cluster"
}

output "scylladbcloud_serverless_cluster_id" {
	value = scylladbcloud_serverless_cluster.example.id
}
