### Enhancements

- `scylladbcloud_cluster_connection`: data changes update the
  connection in place; private link connections, which cannot be
  updated, now fail with a clear error.

### Bug fixes

- `scylladbcloud_cluster_connection`: the `INACTIVE` status value no
  longer carries a trailing space.
