data "scyllacloud_provider" "example" {
  vendor = "AWS"
}

output "aws_provider_id" {
  value = data.scyllacloud_provider.example.id
}
