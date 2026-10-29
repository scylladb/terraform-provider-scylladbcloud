# Add two vector search nodes to a cluster's datacenter.
resource "scylladbcloud_vector_search" "example" {
	cluster_id = 1337
	datacenter = "AWS_US_EAST_1"
	node_count = 2
	node_type  = "r7g.large"
}

output "scylladbcloud_vector_search_status" {
	value = scylladbcloud_vector_search.example.status
}
