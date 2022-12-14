terraform {
	required_providers {
		scylla = {
			source = "registry.terraform.io/scylladb/scylla"
		}
	}
}

provider "scylla" {
	token = "..."
}

resource "scylla_cluster" "scylla" {
	name       = "My Cluster"
	region     = "us-east-1"
	node_count = 3
	node_type  = "i3.xlarge"
	cidr_block = "172.31.0.0/16"

	enable_vpc_peering = true
	enable_dns         = true
}

output "scylla_cluster_id" {
	value = scylla_cluster.scylla.id
}

output "scylla_cluster_datacenter" {
	value = scylla_cluster.scylla.datacenter
}
