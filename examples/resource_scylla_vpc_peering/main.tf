provider "scylla" {
  token = "..."
}

resource "scylla_vpc_peering" "scylla" {
	cluster_id = 10
	datacenter = AWS_US_EAST_1

	peer_vpc_id     = "vpc-1234"
	peer_cidr_block = "192.168.0.0/16"
	peer_region     = "us-east-1"
	peer_account_id = "123"

	allow_cql = true
}

output "scylla_vpc_peering_connection_id" {
	value = scylla_vpc_peering.scylla.connection_id
}
