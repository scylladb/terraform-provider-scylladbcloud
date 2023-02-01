resource "scylladbcloud_allowlist_rule" "home" {
	cluster_id = 1337
	cidr_block = "89.74.148.54/32"
}

output "scylladbcloud_allowlist_rule_id" {
	value = scylladbcloud_allowlist_rule.home.rule_id
}
