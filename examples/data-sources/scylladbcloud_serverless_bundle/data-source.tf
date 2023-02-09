# Fetch connection bundle file for serverless cluster identified by 1337.
data "scylladbcloud_serverless_bundle" "my" {
	cluster_id = 1337
}

output "scylladbcloud_serverless_bundle" {
    sensitive = true
	value     = data.scylladbcloud_serverless_bundle.example.connection_bundle
}
