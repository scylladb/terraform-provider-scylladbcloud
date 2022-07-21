data "scylla_cluster" "example" {
  name = "example"
}

output "cluster_nodes_public_ips" {
  value = data.scylla_cluster.example.all[*].public_ip
}
