terraform {
	required_providers {
		scylladbcloud = {
			source = "registry.terraform.io/scylladb/scylladbcloud"
		}
	}
}

provider "aws" { }

provider "scylladbcloud" {
	token = "..."
}

resource "aws_vpc" "bench" {
	cidr_block = "10.0.0.0/16"
}

resource "scylladbcloud_cluster" "mycluster" {
	name       = "Load Test"
	byoa_id    = 1002
	region     = "us-east-1"
	node_count = 3
	node_type  = "i3.xlarge"
	cidr_block = "172.31.0.0/16"

	enable_vpc_peering = true
	enable_dns         = true
}

data "scylladbcloud_cql_auth" "mycluster" {
	cluster_id = scylladbcloud_cluster.mycluster.cluster_id
}

data "aws_caller_identity" "current" {}

resource "scylladbcloud_vpc_peering" "mycluster" {
	cluster_id = scylladbcloud_cluster.mycluster.cluster_id
	datacenter = scylladbcloud_cluster.mycluster.datacenter

	peer_vpc_id     = aws_vpc.bench.id
	peer_cidr_block = aws_vpc.bench.cidr_block
	peer_region     = "us-east-1"
	peer_account_id = data.aws_caller_identity.current.account_id

	allow_cql = true
}

resource "aws_vpc_peering_connection_accepter" "bench" {
	vpc_peering_connection_id = scylladbcloud_vpc_peering.mycluster.connection_id
	auto_accept               = true
}

resource "aws_route_table" "bench" {
	vpc_id = aws_vpc.bench.id

	route {
		cidr_block = scylladbcloud_cluster.mycluster.cidr_block
		vpc_peering_connection_id = aws_vpc_peering_connection_accepter.bench.vpc_peering_connection_id
	}
}

module "scylla-bench" {
	source    = "github.com/rjeczalik/terraform-aws-scylla-bench"
	username  = data.scylladbcloud_cql_auth.mycluster.username
	password  = data.scylladbcloud_cql_auth.mycluster.password
	seeds     = split(",", data.scylladbcloud_cql_auth.mycluster.seeds)
	instances = 4
	keys      = 1000000000
	limit     = 10000

	depends_on = [aws_route_table.bench]
}
