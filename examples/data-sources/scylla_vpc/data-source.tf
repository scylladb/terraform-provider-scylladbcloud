data "scylla_vpc" "networks" {
  cluster_id = 8
}

output "network_cidrs" {
  value = data.scylla_vpc.networks.all[*].cidr
}
