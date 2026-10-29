data "scylladbcloud_availability_zones" "example" {
	region = "us-east-1"
}

output "zone_ids" {
	value = data.scylladbcloud_availability_zones.example.zone_ids
}
