# Create a cluster on AWS cloud.
resource "scylladbcloud_cluster" "example" {
	name       = "My Cluster"
	cloud      = "AWS"
	region     = "us-east-1"
	node_count = 3
	node_type  = "i3.xlarge"
	cidr_block = "172.31.0.0/16"

	enable_vpc_peering = true
	enable_dns         = true
	
	# Enable encryption at rest (AWS only)
	encryption_at_rest = true
	
	# Replication factor (default is 3)
	replication_factor = 3
}

output "scylladbcloud_cluster_id" {
	value = scylladbcloud_cluster.example.id
}

output "scylladbcloud_cluster_datacenter" {
	value = scylladbcloud_cluster.example.datacenter
}
