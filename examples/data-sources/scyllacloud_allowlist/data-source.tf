data "scyllacloud_cluster" "example" {
  name = "example"
}

data "scyllacloud_allowslist" "example" {
  cluster_id = data.scyllacloud_cluster.example.id
}

output "cluster_allowed_ips" {
  value = data.scyllacloud_allowslist.example.all[*].source_address
}
