data "scylla_provider" "example" {
  vendor = "AWS"
}

output "aws_provider_id" {
  value = data.scylla_provider.example.id
}
