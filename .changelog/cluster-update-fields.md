### Enhancements

- `scylladbcloud_cluster`: `name` changes rename the cluster in place
  instead of recreating it.
- `scylladbcloud_cluster`: new `client_encryption_mode` (`encrypted` or
  `both`).
- `scylladbcloud_cluster`: new `replication_factor`, defaulting to 3.
- `scylladbcloud_cluster`: an update applies every changed field in one
  apply.
- `scylladbcloud_cluster`: new `vector_search` block installs vector
  search nodes when the cluster is created.
