data "scylla_cluster" "example" {
  name = "example"
}

resource "scylla_allowlist_rule" "my_ip" {
  cluster_id     = data.scylla_cluster.example.id
  source_address = "123.45.67.89/32"
}
