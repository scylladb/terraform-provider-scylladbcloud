# End-to-end example for ScyllaDB Datacenter network peering on GCP.
resource "google_compute_network" "app" {
	name = "app"
	auto_create_subnetworks = true
}

resource "scylladbcloud_vpc_peering" "example" {
	cluster_id = 1337
	datacenter = "GCE_US_CENTRAL_1"

	peer_vpc_id     = google_compute_network.app.name
	peer_region     = "us-central1"
	peer_account_id = "exampleproject"

	allow_cql = true
}

resource "google_compute_network_peering" "app" {
	name         = "app-peering"
	network      = google_compute_network.app.self_link
	peer_network = scylladbcloud_vpc_peering.example.network_link
}
