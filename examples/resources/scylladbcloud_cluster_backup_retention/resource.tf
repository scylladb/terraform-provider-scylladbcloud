# Keep the backups of a deleted cluster for 14 days. The cluster must have
# been deleted with backup_retention_days above 0.
resource "scylladbcloud_cluster_backup_retention" "example" {
	cluster_id     = 1337
	retention_days = 14
}

output "scylladbcloud_backup_retention_start_date" {
	value = scylladbcloud_cluster_backup_retention.example.start_date
}
