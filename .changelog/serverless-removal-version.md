### Deprecations

- `scylladbcloud_serverless_cluster` and the `scylladbcloud_serverless_bundle`
  data source will be removed in v2.0.0; their warnings now say so.
  `scylladbcloud_cluster` is the successor, but it creates a new cluster
  with its own settings (region, node type, node count or scaling) and does
  not take over a serverless cluster. Before upgrading to v2.0.0, create a
  `scylladbcloud_cluster`, move your data to it, and remove the serverless
  resource and data source from your configuration. `terraform destroy`
  deletes the serverless cluster; `terraform state rm` only makes Terraform
  forget it, so use it only for a cluster that is already gone or that you
  keep outside Terraform. v2.0.0 fails on state for a resource type it no
  longer knows. Nothing replaces the `connection_bundle` file: the
  `scylladbcloud_cql_auth` data source gives a cluster's connection details
  as separate attributes (seeds, username, password), so change what reads
  the bundle to use them.
