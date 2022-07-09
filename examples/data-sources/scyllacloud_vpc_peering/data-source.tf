data "scyllacloud_cluster" "example" {
  name = "example"
}

data "scyllacloud_vpc_peering" "peerings" {
  cluster_id = data.scyllacloud_cluster.example.id
}

output "cluster_vpc_peers_external_ids" {
  value = data.scyllacloud_vpc_peering.peerings.all[*].peer_vpc_external_id
}
