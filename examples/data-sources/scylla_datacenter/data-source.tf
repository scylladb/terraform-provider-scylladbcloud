data "scylla_cluster" "example" {
  name = "example"
}

data "scylla_datacenter" "dc_us_west_1" {
  cluster_id = data.scylla_cluster.example.id
  name       = "AWS_US_WEST_1"
}

output "datacenter_status" {
  value = data.scylla_datacenter.dc_us_west_1.status
}
