#!/usr/bin/env bash
# Prints dev/datasources.tf for plan row $1 (14-17): every data source that
# row has, read against the regression's cluster.
row="$1"
cat <<'TF'
data "scylladbcloud_account" "dev" {}

output "ds_account_name" {
  value = data.scylladbcloud_account.dev.name
}
TF
((row >= 15)) && cat <<'TF'

data "scylladbcloud_clusters" "dev" {
  depends_on = [scylladbcloud_cluster.dev]
}

data "scylladbcloud_cluster" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
}

data "scylladbcloud_cluster_nodes" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
}

data "scylladbcloud_cluster_datacenters" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
}

output "ds_cluster_node_count" {
  value = data.scylladbcloud_cluster.dev.node_count
}

output "ds_datacenter" {
  value = data.scylladbcloud_cluster_datacenters.dev.datacenters[0].name
}

output "ds_nodes" {
  value = length(data.scylladbcloud_cluster_nodes.dev.nodes)
}

output "ds_clusters_has_dev" {
  value = contains([for c in data.scylladbcloud_clusters.dev.clusters : c.cluster_id], scylladbcloud_cluster.dev.cluster_id)
}
TF
((row >= 16)) && cat <<'TF'

data "scylladbcloud_vpc_peerings" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
}

data "scylladbcloud_cluster_connections" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
}

data "scylladbcloud_cloud_accounts" "dev" {}

output "ds_cloud_accounts" {
  value = length(data.scylladbcloud_cloud_accounts.dev.cloud_accounts)
}

output "ds_peerings" {
  value = length(data.scylladbcloud_vpc_peerings.dev.vpc_peerings)
}
TF
((row >= 17)) && cat <<'TF'

data "scylladbcloud_cloud_providers" "dev" {}

data "scylladbcloud_regions" "dev" {}

data "scylladbcloud_instance_types" "dev" {
  region = "us-east-1"
}

data "scylladbcloud_instance_types" "vector" {
  region = "us-east-1"
  target = "VECTOR_SEARCH"
}

data "scylladbcloud_scylla_versions" "dev" {}

data "scylladbcloud_availability_zones" "dev" {
  region = "us-east-1"
}

data "scylladbcloud_cluster_requests" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
}

output "ds_default_version" {
  value = data.scylladbcloud_scylla_versions.dev.default_version
}

output "ds_has_i8g" {
  value = contains([for i in data.scylladbcloud_instance_types.dev.instance_types : i.name], "i8g.large")
}

output "ds_vector_types" {
  value = length(data.scylladbcloud_instance_types.vector.instance_types)
}

output "ds_zones" {
  value = length(data.scylladbcloud_availability_zones.dev.zone_ids)
}

output "ds_create_request" {
  value = contains([for r in data.scylladbcloud_cluster_requests.dev.requests : r.type], "CREATE_CLUSTER")
}
TF
true
