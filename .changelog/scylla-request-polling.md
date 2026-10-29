### Enhancements

- A failed cluster request now reports its type, how far it got and the
  API's progress description, e.g. `cluster request ID 230229 failed
  (CREATE_CLUSTER at 30%): ...`, instead of only its ID. The public API
  does not expose the internal error behind a failure, such as a cloud
  capacity shortage, so the full reason still needs a ScyllaDB Cloud change.
<<<<<<< HEAD:.changelog/scylla-request-polling.md
=======

### Bug fixes

- `scylladbcloud_cluster`: importing a CQL cluster now stores
  `alternator_write_isolation`; it was left empty, so the first plan after
  `terraform import` replaced the cluster.
### Development
>>>>>>> 50c3d9c (fix(cluster): keep alternator_write_isolation on import):CHANGELOG.md
