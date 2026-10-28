# Plan: cover the Siren-managed public API, and test against a local Siren

Branch: `feat/public-api-coverage`

## Goals

1. Cover every public endpoint (`x-public: true` in siren-frontend's
   `swagger.json`) that Siren manages and the provider does not use yet.
   The following are excluded:
   - `POST /account/{a}/cluster/{c}/notifications/email`
   - `POST /account/{a}/billing/cost`
   - `POST /billing/v1/metering`
   - `POST /pricing`
2. Add a dev mode so the provider can run against a local Siren with no token
   in the configuration.
3. Add a local test stack (Siren on minicloud plus siren-frontend in public
   mode) and acceptance tests that run against it with a local state file.

## Background

The provider calls siren-frontend's public-mode Node.js server
(`api.cloud.scylladb.com`). That server forwards to `siren-public-api`, which
only accepts paths in `public_restapi.endpoints_whitelist`. Every endpoint below
is both documented as public and whitelisted in Siren. Responses come wrapped
in siren-frontend's `{data, error}` envelope.

## Part 1: API coverage

### 1.1 Client methods (`internal/scylla/endpoints.go`)

| Method | Endpoint | Body or response |
|---|---|---|
| `RenameCluster` | `PATCH /account/{a}/cluster/{c}/name` | `{name}` → ClusterRequest |
| `SetClientEncryptionMode` | `PUT /account/{a}/cluster/{c}/encryption/mode` | `{mode: encrypted\|both}` → ClusterRequest |
| `GetBackupRetention` | `GET /account/{a}/cluster/{c}/backup-retention` | `{retentionDays, startDate, encryptionKeys[]}` |
| `UpdateBackupRetention` | `PATCH /account/{a}/cluster/{c}/backup-retention` | `{retentionDays}` |
| `GetVectorSearch` | `GET …/dc/{dc}/vector-search` | `{status, availabilityZones[]}` |
| `CreateVectorSearch` | `POST …/dc/{dc}/vector-search` | `{nodeCount, defaultInstanceTypeId}` |
| `UpdateVectorSearch` | `PATCH …/dc/{dc}/vector-search` | `{nodeCount, defaultInstanceTypeId}` |
| `DeleteVectorSearch` | `DELETE …/dc/{dc}/vector-search` | none |
| `UpdateClusterVPCPeering` | `PUT …/network/vpc/peer/{p}` | `{status, cidrBlock}` |
| `RecreateClusterConnection` | `PUT …/network/vpc/connection/{id}` | `{name, cidrList, data, status}` |
| `GetAccountNotificationEmails` | `GET /account/{a}/notifications/email` | `{emails[]}` |
| `SetAccountNotificationEmails` | `POST /account/{a}/notifications/email` | `{emails[]}` |
| `GetAccountCertificate` | `GET /account/{a}/certificate` | `{content, format}` |

The following methods already exist but nothing calls them. They will be
wired into data sources: `ListClusterNodes`, `ListClusterVPCPeerings`,
`ListClusterConnections`, `ListCloudProviders`, `ListCloudProviderRegions`,
`ListScyllaVersions` and `ListCloudAccounts`.

Add the matching types to `internal/scylla/model/model.go`, and add an
`AwaitClusterRequest` helper for async calls that return a ClusterRequest. The
existing wait loop in `cluster.go:1392` gets moved into that helper.

### 1.2 Changes to `scylladbcloud_cluster`

- **`name` becomes updatable.** Remove `ForceNew`. In `resourceClusterUpdate`,
  when `name` changes, call `RenameCluster` and wait for the request. This
  needs a state upgrade test that proves an existing state shows no diff.
- **New `client_encryption_mode` attribute** (Optional, Computed, values
  `encrypted` or `both`). When it changes, call `SetClientEncryptionMode` and
  wait. Read the value back from the cluster response (confirm the field name
  in the GET `/cluster/{c}` payload first).
- **New `replication_factor` attribute** (Optional, ForceNew, default 3). Today
  `cluster.go:816` hard-codes 3. The default keeps existing configurations
  free of diffs.
- **Restructure `resourceClusterUpdate`.** Today it returns early after the
  scaling branch. It needs to apply every changed field in sequence: rename,
  encryption mode, scaling, then resize.
- `backup_retention_days` stays as is. It is the retention applied when the
  cluster is deleted, and its description will say so explicitly to avoid
  confusion with the new resource.

### 1.3 New resources

| Resource | Endpoints | Notes |
|---|---|---|
| `scylladbcloud_cluster_backup_retention` | GET and PATCH backup-retention | Singleton per cluster, ID = `cluster_id`. Create and Update call PATCH. Delete removes the resource from state only, or optionally resets to a configured value (`reset_on_destroy`). Exposes `start_date` and `encryption_keys`, both computed. Import by cluster ID. |
| `scylladbcloud_vector_search` | vector-search GET, POST, PATCH, DELETE | Attributes: `cluster_id`, `datacenter` (resolved to `dcId` with `ListDataCenters`, like `vpc_peering`), `node_count`, `node_type` (resolved to `defaultInstanceTypeId` through the region instance list), and computed `status` and `availability_zones`. Create and Delete poll GET until the status is HEALTHY or the resource returns 404. Import by `cluster_id:dc_id`. |
| `scylladbcloud_notification_emails` | account GET and POST notifications/email | Singleton per account, `emails` as a set. Delete sends an empty list, but only if the API accepts that; otherwise it removes the resource from state with a warning. |

### 1.4 Existing resources that gain in-place updates

- **`scylladbcloud_vpc_peering`:** `peer_cidr_blocks` drops `ForceNew` and is
  applied with PUT (`cidrBlock`). First confirm in Siren's
  `restapi/cluster_vpc_peer.go#putPeer` that PUT accepts a CIDR change on an
  ACTIVE peering. If it doesn't, keep ForceNew and only document the
  limitation. `allow_cql` is not in the PUT body, so the comment at
  `vpc_peering.go:93` stays.
- **`scylladbcloud_cluster_connection`:** fields that PATCH cannot change but
  PUT (recreate) can, such as `data` or the type-specific blocks, switch from
  ForceNew to a PUT plus a wait for status ACTIVE. Siren's `putConnection`
  defines exactly which fields those are; check it before changing the schema.

### 1.5 New data sources

| Data source | Endpoint |
|---|---|
| `scylladbcloud_account` | `/account/default`, returning `account_id` and `name` |
| `scylladbcloud_account_certificate` | `/account/{a}/certificate` |
| `scylladbcloud_clusters` | `/account/{a}/clusters`, with optional `name_regex` and `status` filters |
| `scylladbcloud_cluster` | `/account/{a}/cluster/{c}`, read-only, by ID or name |
| `scylladbcloud_cluster_nodes` | `/account/{a}/cluster/{c}/nodes` |
| `scylladbcloud_cluster_datacenters` | `/account/{a}/cluster/{c}/dcs` |
| `scylladbcloud_vpc_peerings` | `GET …/network/vpc/peer` |
| `scylladbcloud_cluster_connections` | `GET …/network/vpc/connection` |
| `scylladbcloud_cloud_accounts` | `/account/{a}/cloud-account` (BYOA) |
| `scylladbcloud_availability_zones` | `…/cloud-account/{ca}/region/{r}/zones` |
| `scylladbcloud_cloud_providers` | `/deployment/cloud-providers` |
| `scylladbcloud_regions` | `/deployment/cloud-provider/{p}/regions` |
| `scylladbcloud_instance_types` | `/deployment/cloud-provider/{p}/region/{r}` |
| `scylladbcloud_scylla_versions` | `/deployment/scylla-versions` |
| `scylladbcloud_cluster_requests` | `/account/{a}/cluster/{c}/request` (type and status filters) |

The deployment data sources reuse the provider's `Cloudmeta` cache where
possible, so they make no extra calls.

### 1.6 Not in scope

- **Create-time `maintenanceWindows`.** The API accepts it on create, but the
  maintenance endpoints are private and not whitelisted, so Terraform could
  never read the value back and would show permanent drift. Revisit if Siren
  makes them public.
- **Create-time `vectorSearch` and `promProxy`.** Vector search is covered by
  its own resource instead, so two places don't manage the same thing. The
  promproxy endpoints are private.
- The private endpoints the provider already uses (`/cluster/connect`,
  `/dc/{dc}`) and the broken `serverless_bundle` data source are tracked
  separately.

### 1.7 Docs and examples

For every new resource and data source:
`examples/{resources,data-sources}/<name>/{resource,data-source}.tf`, an
`import.sh` for resources, and templates under `templates/` where they need
custom prose. Then regenerate with `make generate` (tfplugindocs).

## Part 2: dev mode for a local Siren

Locally, siren-public-api still requires a Siren access token: `/account/*`
handlers reject anonymous users, even with `skip_access_token_auth`. In dev
mode the provider gets that token itself, so the user configures none.

### 2.1 Security model

Dev mode is a guard against accidents, not a security boundary. It skips no
authentication: it does what a user can already do by hand, logging in with an
email and password and minting an access token. The provider runs on the
user's machine and is open source, so no check inside it can stop someone who
deliberately bypasses it. Server-side protection belongs to Siren and the
gateway (see 2.6).

The controls below make sure nobody points dev mode at a real environment by
accident, and that dev credentials never leave the machine.

### 2.2 Dev mode only in dev builds

- All dev-mode code lives in files with the `//go:build dev` tag:
  `internal/scylla/devauth.go`, `internal/provider/provider_dev.go` (the
  `dev_*` schema attributes and how they're configured) and their tests.
- `internal/provider/provider_nodev.go` (`//go:build !dev`) provides no-op
  hooks. In a release build, the provider schema has no `dev_*` attributes, and
  a configuration that sets `dev_mode` fails with Terraform's
  "Unsupported argument" error.
- `.goreleaser.yml` builds without the tag. The registry binary can't contain
  dev mode.
- Add a CI check: build the release target and confirm that
  `go tool nm` / `strings` on the binary has no `devauth` symbols and no
  `dev_mode` attribute. Also run a schema test (`go test` without the tag) that
  fails if any `dev_*` key is in `provider.Schema`.
- Local dev builds use `make build-dev`
  (`go build -tags dev -o terraform-provider-scylladbcloud .`). `testacc-local`
  and `dev/` use that binary through `dev_overrides`.

### 2.3 Provider schema (dev builds only)

| Attribute | Env | Default |
|---|---|---|
| `dev_mode` | `SCYLLADB_CLOUD_DEV_MODE` | `false` |
| `dev_siren_endpoint` | `SCYLLADB_CLOUD_DEV_SIREN_ENDPOINT` | `http://localhost:8080/api/v1` |
| `dev_user_email` | `SCYLLADB_CLOUD_DEV_USER_EMAIL` | `tf-dev@local.test` |
| `dev_user_password` (sensitive) | `SCYLLADB_CLOUD_DEV_USER_PASSWORD` | none (see 2.5) |

The `token` check moves from `ValidateDiagFunc`, which can't see other
attributes, into `ConfigureContextFunc`. There, an empty token is an error
only when `dev_mode` is false. In release builds the token is still always
required.

### 2.4 Endpoint guard

These checks run in `ConfigureContextFunc`, before any request is built. A
failure is a hard error with no override. There is no
`SCYLLADB_CLOUD_DEV_ALLOW_REMOTE` or similar escape hatch.

1. `endpoint` and `dev_siren_endpoint` must both be set explicitly. Dev mode
   refuses the default `https://api.cloud.scylladb.com`.
2. Both hosts are resolved with `net.DefaultResolver.LookupIPAddr`, and every
   returned address must be loopback (`127.0.0.0/8`, `::1`). Checking resolved
   addresses, not just the hostname, catches `/etc/hosts` overrides and names
   like `localhost.example.com`.
3. The scheme must be `http` or `https`, and the URL must have no user info.
4. The same check runs again at dial time: the dev HTTP client's
   `DialContext` refuses any non-loopback address. A DNS change between
   configuration and request (DNS rebinding) can't redirect traffic.
5. Containers: `host.docker.internal` is not loopback, so it is rejected.
   Run Terraform on the host, or use host networking (`--net host`) so that
   `localhost` works.

If a lab environment with local CIAM ever needs dev mode, add an explicit
allowlist of exact hosts in a separate, reviewed change. Patterns such as
`*.scylladb.com` or `*.scylla.cloud` will never be accepted.

### 2.5 Credentials

- There is no built-in default password. `scripts/local/stack.sh up`
  generates a random 32-byte password for the dev user and writes it to
  `$XDG_CONFIG_HOME/terraform-provider-scylladbcloud/dev-credentials.json`
  with permissions 0600.
- The provider reads `dev_user_password` from the attribute, the env var or
  that file, in that order. It refuses to read the file if the permissions are
  looser than 0600.
- The dev user's email is in the reserved `.test` domain
  (`tf-dev@local.test`), so it can't match a real account.
- The password is only ever sent to `dev_siren_endpoint`, and only after the
  guard in 2.4 passes.

### 2.6 Token flow (`internal/scylla/devauth.go`)

1. `POST {dev_siren_endpoint}/user/login` with `{email, password}` returns a
   session token (Siren `restapi/user.go#login`, local CIAM).
2. `POST {dev_siren_endpoint}/user/access-token` with the session token and
   `{Description: "terraform-dev", ExpirationDate: now+24h}` returns
   `{Token}`. This is the same flow as Siren's e2e `LegacyPublicAPIToken`.
3. `Client.Token` is set to the access token, and requests go to `endpoint`
   (siren-frontend public mode) through the loopback-only HTTP client.
4. Cache the token in `$XDG_CACHE_HOME/terraform-provider-scylladbcloud/dev-token.json`
   (0600), keyed by endpoint and email, until it expires. A cached token is
   reused only for the endpoint it was minted for.

### 2.7 Visibility

- Every run in dev mode logs a `WARN` diagnostic: "dev mode is enabled; using
  a local Siren at <endpoint>".
- Every request carries `X-Scylla-Dev-Mode: 1` and the user agent gets the
  suffix `dev`. Any dev-mode traffic that reached a real environment would be
  easy to spot in gateway logs.

### 2.8 Server side (Siren, separate from this branch)

To be raised with the Cloud API squad, not implemented here:

- Production has `features.frontegg: both`
  (`siren/config/files/production.yaml:258`), so Siren's local-login path is
  turned on. Confirm whether the gateway exposes `/api/v1/user/login` and
  `/api/v1/user/access-token` to the internet.
- If it does, block local-CIAM login and token minting at the gateway in
  production. That protects against any client, not just Terraform.

### 2.3 Local state

The provider needs no code change for this. Terraform's default `local` backend
works as is. Add a `dev/` workspace:

- `dev/main.tf`: `terraform { backend "local" { path = "terraform.tfstate" } }`
  and a `provider "scylladbcloud" { dev_mode = true, endpoint = "http://localhost:3011" }`
  block, plus a small cluster that fits minicloud.
- `dev/.terraformrc.tmpl`: `dev_overrides` pointing at the repo root, the same
  approach as Siren's `docs/DEVELOPING_TERRAFORM_PROVIDER.md`.
- Add `dev/*.tfstate*`, `dev/.terraform*` and `dev/.terraformrc` to
  `.gitignore`.

## Part 3: testing

### 3.1 Unit tests (no network, run in CI)

- Add `internal/scylla/scyllatest`: an `httptest.Server` fake of
  siren-frontend's public API with the `{data, error}` envelope, in-memory
  clusters, DCs, peers, connections, vector search, retention and emails, and
  ClusterRequest progress. It is driven by a small router table.
- Add client tests for every new method: paths, verbs, bodies, envelope
  decoding and error mapping.
- Add `resource.UnitTest` cases for every new resource and data source against
  the fake server. These cover create, read, update, import, plan-only
  no-drift and destroy. They need the `terraform` binary but no Siren.
- Add dev-auth tests (`-tags dev`): login, access-token, the cache and its
  permissions, and the rule that a token is required only when dev mode is
  off.
- Add endpoint guard tests (`-tags dev`):
  - Rejected: the default endpoint, a non-loopback IP, a hostname that
    resolves to a non-loopback address (fake resolver), `host.docker.internal`,
    user info in the URL, and a dial-time address change (fake dialer).
  - Accepted: `localhost`, `127.0.0.1` and `[::1]`.
- Add release-build tests (no tag): the provider schema has no `dev_*`
  attributes, and a configuration that sets `dev_mode` fails to validate.
- Add state-upgrade tests: a v2 cluster state from the released provider shows
  an empty plan after `name` loses `ForceNew` and the new attributes are added.

### 3.2 Local stack (Siren on minicloud)

Add `scripts/local/stack.sh up|down|status`, which runs the following steps.

1. **Siren and minicloud.** In `../siren`, run
   `MINICLOUD_RESTART=0 AUTO_APPROVE=1 STAGES=none make all-local`. Its
   preflight starts minicloud (`:5050`), MySQL, Vault, the fake BYOK services
   and Siren in Docker, with restapi on `:8080` and public-api on `:8088`, using
   `scylla-dbaas.yml.minicloud-docker`. Verify that `STAGES=none` runs only the
   preflight. If it doesn't, call `scripts/local/preflight.sh` directly.
2. **siren-frontend in public mode.** In `../siren-frontend/server`, run
   `PUBLIC_API=true LOCAL_AUTH=true API_BACKEND_URL=http://localhost:8088/api/v0 API_WEBHOOK_URL=http://localhost:8088 SERVER_PORT=3011 pnpm start`.
   This is the provider's `endpoint`. `LOCAL_AUTH` makes siren-frontend accept
   Siren-local tokens instead of Frontegg JWTs.
3. **Dev user bootstrap**, idempotent:
   - Generate a random password, or reuse the one in
     `dev-credentials.json` (0600, see 2.5).
   - `POST :8080/api/v1/user` creates `tf-dev@local.test`.
   - `./scylla-cloud user verify --email tf-dev@local.test --config scylla-dbaas.yml.minicloud`.
   - Log in once to confirm a default account exists.
4. **Health check.** `GET :3011/deployment/cloud-providers` and
   `GET :3011/account/default` must both return 200 with a dev token.
5. `down` stops siren-frontend and runs `make all-local-down` in `../siren`.

Repo paths come from `SIREN_DIR`, `SIREN_FRONTEND_DIR` and `MINICLOUD_DIR`,
which default to `../siren`, `../siren-frontend` and `../minicloud`.

### 3.3 Acceptance tests against the local stack

- Add Makefile targets:
  - `build-dev`: `go build -tags dev`.
  - `testacc-local`: `SCYLLADB_CLOUD_DEV_MODE=1 SCYLLADB_CLOUD_ENDPOINT=http://localhost:3011 TF_ACC=1 go test -tags dev -parallel=1 -timeout=120m …`.
- `testAccPreCheck` accepts dev mode instead of `SCYLLADB_CLOUD_TOKEN`.
- Tests run serially (`-parallel=1`): on macOS, minicloud runs in
  `--lightweight` mode and can't hold more than one cluster's VMs.
- Every test uses terraform-plugin-testing's default local state in a temporary
  working directory. One extra test drives `dev/` with the real `terraform`
  CLI (init, apply, plan with `-detailed-exitcode` == 0, destroy) to check the
  local backend workflow end to end.
- Test fixtures are AWS clusters in a minicloud region with an ARM64 instance
  type on Apple Silicon (`E2E_ARM64_INSTANCE_TYPE`, default `i8g.large`). They
  read Scylla versions from the `scylladbcloud_scylla_versions` data source
  instead of hard-coding them.

Minicloud only proves `TestCreateCluster` today
(`siren/scripts/local/e2e-minicloud.tests`). Tests are grouped so unknown
support is explicit:

| Group | Tests | Expected on minicloud |
|---|---|---|
| A: core | cluster create, read, import, rename, encryption mode; allowlist rules; backup retention; notification emails; all data sources | Should pass |
| B: networking | VPC peering create, update (CIDR), delete; cluster connection PUT | Unknown. Gate behind `SCYLLADB_CLOUD_LOCAL_NETWORKING=1` |
| C: vector search | create, resize, delete | Unknown (needs vector-store images). Gate behind `SCYLLADB_CLOUD_LOCAL_VECTOR=1` |

Groups B and C keep running against lab or staging, with a real token, through
the existing `make testacc`.

### 3.4 CI

- `.github/workflows/go.yml`: run unit tests, including the fake-server
  `resource.UnitTest` cases, on every PR. Install the `terraform` CLI with
  `hashicorp/setup-terraform`.
- Run the tests twice: once without tags (release behaviour, including the
  no-`dev_*` schema test) and once with `-tags dev` (dev auth and the
  endpoint guard).
- Add a release-binary check: build exactly as `.goreleaser.yml` does, and
  fail if the binary contains dev-mode symbols or the `dev_mode` attribute.
- `testacc-local` stays manual, because minicloud needs KVM or QEMU and cached
  AMIs. Document it in `CONTRIBUTING.md`.

## Milestones

1. **Dev mode and local stack**: Part 2, including the build tag, the
   endpoint guard, the credentials file and the release-binary CI check. Plus
   `stack.sh` and a `testacc-local` run of the existing basic AWS cluster
   test. This proves the harness and the safety controls before any new
   feature lands.
2. **Fake server and client methods**: 3.1 infrastructure and 1.1.
3. **Cluster changes**: 1.2 (rename, client encryption, replication factor),
   with state-upgrade tests.
4. **New resources**: backup retention, notification emails, vector search
   (1.3).
5. **In-place updates for peering and connections** (1.4), after checking
   Siren's PUT semantics.
6. **Data sources** (1.5).
7. **Docs and examples** (1.7), plus a CHANGELOG entry.

Each milestone ends green on `make lint test` and on Group A of
`testacc-local`.

## Open items to verify during implementation

- The field name for client encryption mode in GET `/cluster/{c}`.
- Whether `POST notifications/email` accepts an empty list.
- Whether `PUT vpc/peer/{p}` with a new `cidrBlock` works on an ACTIVE peering.
- Which connection fields `putConnection` can change.
- Whether `STAGES=none make all-local` stops after the preflight.
- The response shape of `/user/login`, specifically which field holds the
  session token.
- Whether Siren's `createUser` accepts a `.test` email domain in the
  minicloud config (`allowed_registration_domains` defaults to
  `scylladb.com`). If it doesn't, add `local.test` to that list in the
  generated `scylla-dbaas.yml.minicloud*` files rather than using a real
  domain.
