data "scyllacloud_cluster" "example" {
  name = "example"
}

data "scyllacloud_allowslist_rule" "example" {
  cluster_id     = data.scyllacloud_cluster.example.id
  source_address = "83.23.117.37/32"
}

output "cluster_allowed_ip_id" {
  value = data.scyllacloud_allowslist_rule.example.id
}
