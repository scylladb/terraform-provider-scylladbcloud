### Deprecations

- `scylladbcloud_serverless_cluster` and the `scylladbcloud_serverless_bundle`
  data source will be removed in v2.0.0; their warnings now say so. Before
  upgrading to v2.0.0, move to `scylladbcloud_cluster` (and the
  `scylladbcloud_cql_auth` data source for connection details), then run
  `terraform state rm` on the old resource and data source addresses so that
  v2.0.0 does not find state for a resource type it no longer knows.
