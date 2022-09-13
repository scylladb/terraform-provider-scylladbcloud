
data "scylla_provider" "aws_provider" {
  vendor = "AWS"
}

data "scylla_provider_region" "available" {
  provider_id = data.scylla_provider.aws_provider.id
}

data "scylla_provider_instance" "available" {
  provider_id = data.scylla_provider.aws_provider.id
}

resource "scylla_cluster" "production" {
  provider_id        = data.scylla_provider.aws_provider.id
  provider_region_id = data.scylla_provider_region.available.all["us-west-2"].id
  instance_type_id   = data.scylla_provider_instance.available.all["t3.micro"].id

  user_api_interface = "CQL"
  broadcast_type     = "PRIVATE" # PRIVATE | PUBLIC ; vpc_peering_enabled
  name               = "production-1"
  replication_factor = 3
  nodes              = 3
  cidr               = "172.31.0.0/16"
}
