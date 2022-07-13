data "scylla_cluster" "example" {
  name = "example"
}

data "scylla_allowslist" "example" {
  cluster_id = data.scylla_cluster.example.id
}

output "cluster_allowed_ips" {
  value = data.scylla_allowslist.example.all[*].source_address
}
