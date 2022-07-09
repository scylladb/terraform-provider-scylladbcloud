data "scyllacloud_cluster" "example" {
  name = "example"
}

data "scyllacloud_vpc" "vpcs" {
  cluster_id = data.scyllacloud_cluster.example.id
}

output "cluster_vpcs_cidrs" {
  value = data.scyllacloud_vpc.vpcs.all[*].ipv4_cidr
}
