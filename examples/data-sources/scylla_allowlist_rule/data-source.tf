data "scylla_cluster" "example" {
  name = "example"
}

data "scylla_allowslist_rule" "example" {
  cluster_id     = data.scylla_cluster.example.id
  source_address = "83.23.117.37/32"
}

output "cluster_allowed_ip_id" {
  value = data.scylla_allowslist_rule.example.id
}
