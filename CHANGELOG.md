# Changelog

## Unreleased

Configurations and state written by v1.13.0 keep working unchanged:
upgrading produces no plan changes.

### New resources

- `scylladbcloud_vector_search`: vector search nodes on a cluster
  datacenter.
- `scylladbcloud_notification_emails`: the account's notification email
  list.
- `scylladbcloud_cluster_backup_retention`: how long a cluster's backups
  are kept.

### New data sources

- Account: `scylladbcloud_account`, `scylladbcloud_account_certificate`.
- Clusters: `scylladbcloud_clusters`, `scylladbcloud_cluster`,
  `scylladbcloud_cluster_nodes`, `scylladbcloud_cluster_datacenters`,
  `scylladbcloud_cluster_requests`.
- Networking: `scylladbcloud_vpc_peerings`,
  `scylladbcloud_cluster_connections`, `scylladbcloud_cloud_accounts`.
- Deployment: `scylladbcloud_cloud_providers`, `scylladbcloud_regions`,
  `scylladbcloud_instance_types`, `scylladbcloud_scylla_versions`,
  `scylladbcloud_availability_zones`.

### Enhancements

- `scylladbcloud_cluster`: `name` changes rename the cluster in place
  instead of recreating it.
- `scylladbcloud_cluster`: new `client_encryption_mode` (`encrypted` or
  `both`).
- `scylladbcloud_cluster`: new `replication_factor`, defaulting to 3.
- `scylladbcloud_cluster`: an update applies every changed field in one
  apply.
- `scylladbcloud_vpc_peering`: `peer_cidr_blocks` changes update the
  peering in place instead of recreating it.
- `scylladbcloud_cluster_connection`: data changes update the
  connection in place; private link connections, which cannot be
  updated, now fail with a clear error.

### Deprecations

- `scylladbcloud_serverless_cluster` and the `scylladbcloud_serverless_bundle`
  data source will be removed in v2.0.0; their warnings now say so. Before
  upgrading to v2.0.0, move to `scylladbcloud_cluster` (and the
  `scylladbcloud_cql_auth` data source for connection details), then run
  `terraform state rm` on the old resource and data source addresses so that
  v2.0.0 does not find state for a resource type it no longer knows.

### Bug fixes

- `scylladbcloud_cluster`: importing a CQL cluster now stores
  `alternator_write_isolation`; it was left empty, so the first plan after
  `terraform import` replaced the cluster.
- `scylladbcloud_allowlist_rule`: importing a rule now sets `rule_id`.
- Listing clusters no longer fails when the account holds a cluster whose
  creation failed; that broke the clusters data source and reading
  allowlist rules and VPC peerings.
- `scylladbcloud_cql_auth`: the v0 state upgrader no longer panics on state
  read from JSON or on a datacenter matched by ID.
- `scylladbcloud_cluster_connection`: the `INACTIVE` status value no
  longer carries a trailing space.

### Development

- Builds with `-tags dev` add a dev mode that logs in to a local Siren
  stack (`make build-dev`, `make test-dev`).
- `scripts/local/stack.sh` and `make testacc-local` run the acceptance
  tests against a local stack.
