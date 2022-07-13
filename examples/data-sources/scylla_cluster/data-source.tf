data "scylla_cluster" "example" {
  name = "example"
}

output "cluster_example_scylla_version" {
  value = data.scylla_cluster.example.scylla_version
}
