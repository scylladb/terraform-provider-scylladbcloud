# Create a VPC peering connection to the specified datacenter.
resource "scylladbcloud_vpc_peering" "example" {
	cluster_id = 1337
	datacenter = "AWS_US_EAST_1"

	peer_vpc_id      = "vpc-1234"
	peer_cidr_blocks = ["192.168.0.0/16"]
	peer_region      = "us-east-1"
	peer_account_id  = "123"

	allow_cql = true
}

output "scylladbcloud_vpc_peering_connection_id" {
	value = scylladbcloud_vpc_peering.example.connection_id
}
