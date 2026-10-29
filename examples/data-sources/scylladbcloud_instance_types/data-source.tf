# Instance types for cluster nodes in us-east-1.
data "scylladbcloud_instance_types" "example" {
	region = "us-east-1"
}

# Instance types for vector search nodes.
data "scylladbcloud_instance_types" "vector" {
	region = "us-east-1"
	target = "VECTOR_SEARCH"
}
