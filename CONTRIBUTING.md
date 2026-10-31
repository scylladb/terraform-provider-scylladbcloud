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
- Use clear and descriptive commit messages.
- Make sure you ran linters before submitting your code:
  - `goimports -w ./`
  - `gofmt -s -w ./`
  - `golangci-run run ./...`
- Create a pull request, follow the PR template to fill the description.
- The maintainers will review your pull request and may request changes or ask questions.
    - <details><summary>The `git log` of you PR is also a subject of review, and you could be asked to fix it by squashing with a meaningful message or simply amending commits in the way that you see it fit.</summary>

      Try to maintain a clean `git log` within your branch.
      You may separate your changes into several commits to make the PR reviews easier.

      If you push small commits addressing review comments to fix a small bug or typo, those should be squashed before merging.
      It helps to think of you commits as something that will be easy to revert later, or bisect to identify bugs.
      A commit for `some change` will be easier to handle than 3 commits about `some change` + `small fix` + `fix typo`.

      </details>
    - Try to rebase your PR against main on a regular basis.
    - Try to avoid merging in main your branch instead of rebasing.
- Merge your PR after getting at least one approval (two are preferable on larger PRs).

# Developing Against a Local Stack

Builds with the `dev` build tag add a dev mode that talks to a local Siren
stack instead of ScyllaDB Cloud. Release builds never contain it;
`make check-release` fails if dev mode code reaches the release binary.

1. Start the stack with `scripts/local/stack.sh up`. It needs the `siren`
   and `siren-frontend` repositories next to this one (override with
   `SIREN_DIR` and `SIREN_FRONTEND_DIR`) and creates a verified dev user whose
   credentials it saves under `~/.config/terraform-provider-scylladbcloud`.
2. Install the dev build with `make build-dev`. It goes to `$GOPATH/bin`;
   point Terraform at it with a `dev_overrides` block in your CLI config.
3. Point the provider at the stack's public API and turn dev mode on:

   ```bash
   export SCYLLADB_CLOUD_ENDPOINT=http://localhost:3011
   export SCYLLADB_CLOUD_DEV_MODE=1
   ```

   Dev mode logs in with the saved credentials, so no token is needed. It only
   accepts loopback endpoints. `SCYLLADB_CLOUD_DEV_SIREN_ENDPOINT` overrides
   the Siren API address (default `http://localhost:8080/api/v1`).

`make testacc-local` runs acceptance tests against the stack the same way;
pick tests with `run=`. Stop the stack with `scripts/local/stack.sh down`.

# Running Tests

## Unit Tests

Run unit tests with:

```bash
make test
```

## Acceptance Tests

Acceptance tests create real resources in ScyllaDB Cloud, so they require valid credentials and may incur costs.

### Required Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `SCYLLADB_CLOUD_TOKEN` | API token for authentication | Yes |
| `SCYLLADB_CLOUD_ENDPOINT` | Custom API endpoint (optional) | No |
| `SCYLLADB_CLOUD_GCP_BYOA_ID` | GCP BYOA credential ID for BYOA tests | For GCP BYOA tests only |

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
