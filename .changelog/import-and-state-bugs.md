### Bug fixes

- `scylladbcloud_cluster`: importing a CQL cluster now stores
  `alternator_write_isolation`; it was left empty, so the first plan after
  `terraform import` replaced the cluster.
- `scylladbcloud_allowlist_rule`: importing a rule now sets `rule_id`.
