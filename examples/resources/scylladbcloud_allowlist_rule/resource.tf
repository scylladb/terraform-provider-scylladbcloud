# Add a CIDR block to allowlist for the specified cluster.
resource "scylladbcloud_allowlist_rule" "example" {
	cluster_id = 1337
	cidr_block = "89.74.148.54/32"
}

output "scylladbcloud_allowlist_rule_id" {
	value = scylladbcloud_allowlist_rule.example.rule_id
}
