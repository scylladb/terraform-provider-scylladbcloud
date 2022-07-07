
data "scyllacloud_cluster" "example" {
  name = "example"
}

output "cluster_example_scylla_version" {
  value = data.scyllacloud_cluster.example.scylla_version
}
