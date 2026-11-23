# A VPC peering can be imported by its connection ID.
terraform import scylladbcloud_vpc_peering.example pcx-0123456789abcdef0

# The API does not return the region of a GCP peering, so the import ID of a
# GCP peering also takes its peer region: <connection_id>,<peer_region>.
terraform import scylladbcloud_vpc_peering.example np-1-0123abcd-peering-0123abcd,us-central1

# A GCP peering imported with an earlier provider version has an empty
# peer_region in its state. If the next plan replaces it because of
# peer_region, note its connection_id, remove it from the state, and import
# it again right away, at the same address and with its peer region. The
# plan must then not replace it because of peer_region, but it can still
# show other changes.
terraform state show scylladbcloud_vpc_peering.example
terraform state rm scylladbcloud_vpc_peering.example
terraform import scylladbcloud_vpc_peering.example np-1-0123abcd-peering-0123abcd,us-central1
terraform plan

# Keep peer_cidr_blocks in the configuration of an AWS peering, and of a GCP
# peering with custom CIDR blocks. Without the list, the next plan fails for
# AWS. For GCP, it fails when the region has no default CIDR block, and
# otherwise replaces the peering with one on that block. Before it plans the
# replacement, it checks that the cluster is on GCP and, when the plan knows
# the datacenter name, that the cluster has that datacenter.
