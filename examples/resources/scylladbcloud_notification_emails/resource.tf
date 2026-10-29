# Send the account's notifications to two addresses.
resource "scylladbcloud_notification_emails" "example" {
	emails = ["ops@example.com", "oncall@example.com"]
}
