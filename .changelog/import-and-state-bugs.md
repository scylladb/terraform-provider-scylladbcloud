### Bug fixes

- `scylladbcloud_cluster`: importing a CQL cluster now stores
  `alternator_write_isolation`; it was left empty, so the first plan after
  `terraform import` replaced the cluster.
- `scylladbcloud_allowlist_rule`: importing a rule now sets `rule_id`.
- Listing clusters no longer fails when the account holds a cluster whose
  creation failed; that broke reading allowlist rules and VPC peerings.
- `scylladbcloud_cql_auth`: the v0 state upgrader no longer panics on state
  read from JSON or on a datacenter matched by ID.
