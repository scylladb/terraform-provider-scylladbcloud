### Enhancements

- A failed cluster request now reports its type, how far it got and the
  API's progress description, e.g. `cluster request ID 230229 failed
  (CREATE_CLUSTER at 30%): ...`, instead of only its ID. The public API
  does not expose the internal error behind a failure, such as a cloud
  capacity shortage, so the full reason still needs a ScyllaDB Cloud change.
