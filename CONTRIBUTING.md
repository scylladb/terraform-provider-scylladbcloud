# Code of Conduct

We are committed to creating a welcoming and inclusive environment for all contributors.
As such, we have adopted the following code of conduct, which all contributors are expected to follow:

- Be respectful of others. Harassment or discrimination of any kind will not be tolerated.
- Be mindful of your language and behavior when communicating with others.
- Be open-minded and willing to learn from others.

# Contributing Guidelines

We welcome contributions to this repository. Here are some guidelines to follow when contributing:

- Make sure your code follows the [Effective Go](https://golang.org/doc/effective_go.html).
- Run the tests and make sure they pass before submitting your changes.
- Make sure you ran the formatter and the linters before submitting your code:
  - `make fmt`
  - `make lint`
- Create a pull request, follow the PR template to fill the description.
- The maintainers will review your pull request and may request changes or ask questions.
    - Try to rebase your PR against main on a regular basis.
    - Try to avoid merging in main your branch instead of rebasing.
- Merge your PR after getting at least one approval (two are preferable on larger PRs).

## PR titles

PRs are squash-merged. The PR title becomes the commit subject on `main`,
and the PR description becomes the commit body. Commits inside the PR do
not reach `main`, so you can push review fixes as separate commits.

The PR title follows
[Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): subject
```

- **type** is one of:

  | Type | Use it for | Version bump |
  |------|------------|--------------|
  | `feat` | A new resource, data source, field or behaviour | minor |
  | `fix` | A bug fix | patch |
  | `docs` | Docs, examples and templates only | none |
  | `build` | Dependencies, Makefile, GoReleaser | none |
  | `ci` | GitHub workflows | none |
  | `test` | Tests only | none |
  | `refactor` | A code change with no change in behaviour | none |
  | `chore` | Other maintenance | none |
  | `revert` | A revert of an earlier PR | depends |

  Use `feat`, not `feature`.
- **scope** is optional. Use the name of the package directory that
  changes: `cluster`, `vpcpeering`, `allowlistrule`, `connection`,
  `cqlauth`, `serverless`, `stack`, `scylla` (the API client) or
  `provider`. Use `deps` for dependency updates.
- **subject** is in the imperative mood ("add", not "added"). It starts
  with a lowercase letter and has no period at the end. Aim for 50
  characters or fewer for the full title. CI doesn't check the length.
- A breaking change has `!` before the colon, for example
  `feat(cluster)!: remove node_count`. See rule 11 below.

## Review rules

The main rule: an upgrade must never surprise an existing user. The author
checks each rule before asking for review. The reviewer checks them again.

1. **Upgrade plans "No changes".** A resource made by the last release
   must plan "No changes" with the new build. A new field on an existing
   resource is Optional. Make it also Computed only when the API sets a
   value the user did not set. A Computed field keeps its last value when
   the user removes it from the configuration.
2. **Old state keeps working.** If you rename a field or change its type,
   increase `SchemaVersion` by 1 and add a state upgrader that converts the
   old state.
3. **No ForceNew means a real Update.** Each field is ForceNew, or Update
   sends the new value to the API. An Update that does nothing or always
   fails is a bug.
4. **The docs say when a change replaces the resource.** Put it in the
   field's `Description`.
5. **Gone on the server means gone from state.** When the API says that the
   object does not exist, Read calls `d.SetId("")` and Delete returns no
   error.
6. **Fail in plan, not in apply.** Check values with `ValidateDiagFunc`,
   `ConflictsWith` or `CustomizeDiff` where you can.
7. **Save the ID early.** Call `d.SetId` as soon as the API returns the ID,
   before you wait for the object to be ready.
8. **Handle every status.** A wait loop handles each status that the API can
   return and gives a clear message for each.
9. **Tests come in the same PR.** A change in behaviour has unit tests,
   against a fake API where the code calls the API. A new resource
   also has an acceptance test with an import step. A bug fix has a test
   that fails without the fix.
10. **One PR makes one change.** One feature or one bug fix, with its tests
    and docs. Try to stay below about 400 changed lines, not counting
    generated docs. Stacked PRs are fine when each one has one purpose.
11. **Breaking changes only in a major version.** First warn in one
    release. Put `!` in the PR title and add a before and after
    example to the changelog entry.
12. **Docs come from the schema.** Do not edit `docs/` by hand. Change the
    `Description`, `examples/` or `templates/`, run `make generate`, and
    commit the result. CI fails when `docs/` does not match.
13. **Every user-visible change has a changelog entry.** See
    [Changelog](#changelog).

## Changelog

Each PR that changes what users see adds a changelog entry: a change to a
resource, a data source, a field, the provider behaviour or a deprecation.
The release notes of the next version are made from these entries. A PR
that changes only tests, CI or internal code needs no entry. Add the
`no-changelog` label to it instead. CI checks for the entry or the label.
Dependabot PRs are skipped.

Put the entry in `.changelog/<PR number>.txt`. Open the PR first to get
the number. Each entry is a fenced block. The type of the block is one of:

- `release-note:breaking-change`
- `release-note:new-resource`
- `release-note:new-data-source`
- `release-note:feature`
- `release-note:enhancement`
- `release-note:bug`
- `release-note:note`

Start the text with the resource or data source name, then a colon. Write
for the user: say what changed and what the user must do. Example:

````markdown
```release-note:bug
resource/scylladbcloud_cluster: Importing a CQL cluster no longer causes a replace on the next plan
```
````

One file can have more than one block.

## Generated-only resources

`scylladbcloud_stack` exists only for the GCP BYOA and BYOK templates that
`siren-byok` generates. It has no docs page, examples or acceptance tests on
purpose. Do not remove it or change its schema, token format or provider
arguments (`endpoint`, `metadata`, `trace`) without the Cloud BYOx squad.

# Running Tests

## Unit Tests

Run unit tests with:

```bash
make test
```

## Acceptance Tests

Acceptance tests create real resources in ScyllaDB Cloud, so they require valid credentials and may incur costs.

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `SCYLLADB_CLOUD_TOKEN` | API token for authentication | Yes |
| `SCYLLADB_CLOUD_ENDPOINT` | API endpoint, for example `https://api.cloud.scylladb.com` | Yes |
| `SCYLLADB_CLOUD_TRACE` | Trace ID that the API calls send. Without it, each run makes a new one | No |
| `TEST_SCYLLADB_CLOUD_GCP_BYOA_ID` | GCP BYOA credential ID | For the GCP BYOA tests only |
| `TEST_SCYLLADB_CLOUD_EAR_KEY_ID` | Customer-managed key ID for encryption at rest | For the encryption key tests only |

### Running Acceptance Tests

Run all acceptance tests:

```bash
make testacc
```

Run a specific test:

```bash
make testacc run=TestAccScyllaDBCloudCluster_basicAWS
```

List all available acceptance tests:

```bash
make list-acc-tests
```
