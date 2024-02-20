# Create AWS TGW VPC Attachment Connection for the specified cluster.

resource "scylladbcloud_cluster_connection" "aws_tgw" {
	cluster_id = 1337
	name       = "aws-tgw-test"
	cidrlist = ["10.201.0.0/16"]
	type = "AWS_TGW_ATTACHMENT"
	datacenter = "AWS_US_EAST_1"
	data = {
		tgwid  = "tgw-08461afa1119f390a"
 		ramarn = "arn:aws:ram:us-east-1:043400831220:resource-share/be3b0395-1782-47cb-9ae4-6d3517c6a721"
	}
}

output "scylladbcloud_cluster_connection_id" {
	value = scylladbcloud_cluster_connection.aws_tgw.id
}
