
data "scyllacloud_cluster" "example" {
  name = "example"
}

data "scyllacloud_datacenter" "dc_us_west_1" {
  cluster_id = data.scyllacloud_cluster.example.id
  name       = "AWS_US_WEST_1"
}

output "datacenter_status" {
  value = data.scyllacloud_datacenter.dc_us_west_1.status
}
