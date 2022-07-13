data "scylla_provider" "aws_provider" {
  vendor = "AWS"
}

data "scylla_provider_instance" "available" {
  provider_id = data.scylla_provider.aws_provider.id
}

output "t3micro_instance_memory" {
  value = data.scylla_provider_instance.available.all["t3.micro"].memory_mb
}
