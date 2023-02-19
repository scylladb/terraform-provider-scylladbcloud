# End-to-end example for ScyllaDB Datacenter VPC peering on AWS.
resource "aws_vpc" "app" {
	cidr_block = "10.0.0.0/16"
}

data "aws_caller_identity" "current" {}

resource "scylladbcloud_vpc_peering" "example" {
	cluster_id = 1337
	datacenter = "AWS_EAST_1"

	peer_vpc_id     = aws_vpc.app.id
	peer_cidr_block = aws_vpc.app.cidr_block
	peer_region     = "us-east-1"
	peer_account_id = data.aws_caller_identity.current.account_id

	allow_cql = true
}

resource "aws_vpc_peering_connection_accepter" "app" {
    vpc_peering_connection_id = scylladbcloud_vpc_peering.example.connection_id
    auto_accept               = true
}

resource "aws_route_table" "bench" {
	vpc_id = aws_vpc.app.id

	route {
		cidr_block = scylladbcloud_cluster.example.cidr_block
		vpc_peering_connection_id = aws_vpc_peering_connection_accepter.app.vpc_peering_connection_id
	}
}
