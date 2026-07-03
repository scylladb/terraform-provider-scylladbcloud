resource "scylladbcloud_cluster" "standard" {
  name       = "va-tf-test-cluster"
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "t3.micro"
  min_nodes  = 3
  cidr_block = "10.0.1.0/24"
}

resource "scylladbcloud_vector_search" "vs" {
  cluster_id = scylladbcloud_cluster.standard.id

  node_count = 2
  node_type  = "t4g.medium"
}

