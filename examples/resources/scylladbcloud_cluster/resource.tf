# Create a cluster on AWS cloud.
resource "scylladbcloud_cluster" "example" {
	name       = "My Cluster"
	cloud      = "AWS"
	region     = "us-east-1"
	min_nodes  = 3
	node_type  = "i3.xlarge"
	cidr_block = "172.31.0.0/16"

	enable_vpc_peering = true
	enable_dns         = true

	# Accept only TLS client connections. Use "both" to also accept plain
	# connections. Changing it updates the cluster in place.
	client_encryption_mode = "encrypted"

	# Replication factor of the cluster's keyspaces; defaults to 3. Changing
	# it recreates the cluster.
	replication_factor = 3

	# Optional: install vector search nodes when the cluster is created. The
	# block is used only at creation; use scylladbcloud_vector_search to add or
	# resize vector search later.
	#
	#   vector_search {
	#     node_count = 2
	#     node_type  = "r7g.large"
	#   }

	# Encryption at rest is enabled by default using a key managed by ScyllaDB Cloud.
	# This block is optional and should only be configured if you wish to use a
	# customer-managed key previously created in the ScyllaDB Cloud portal.
	#
	#   encryption_at_rest {
	#     key_id = "key-deadbeef"
	#   }
	#
}

output "scylladbcloud_cluster_id" {
	value = scylladbcloud_cluster.example.id
}

output "scylladbcloud_cluster_datacenter" {
	value = scylladbcloud_cluster.example.datacenter
}

output "scylladbcloud_cluster_ca_certificate" {
	value = scylladbcloud_cluster.example.ca_certificate
}

output "scylladbcloud_cluster_encryption_key_provider" {
	value = scylladbcloud_cluster.example.encryption_at_rest[0].provider
}
