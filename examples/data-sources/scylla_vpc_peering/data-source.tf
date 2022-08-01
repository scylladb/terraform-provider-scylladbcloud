data "scylla_vpc_peering" "peering" {
  cluster_id = 16
}

output "peering_external_id" {
  value = data.scylla_vpc_peering.peering.all[0].external_id
}
