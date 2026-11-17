---
name: terraform-provider-qa
description: QA cycle for the ScyllaDB Cloud Terraform provider - check prerequisites, pick or create a dashboard artifact, scan the PRs it lists (review threads, comments, CI) and their Jira issues, fix what needs fixing, run the provider's local regression against Siren on minicloud in dev mode, push, reply/resolve, and update the dashboard. Use when asked to "QA the provider", "check the provider PRs", "run a provider QA cycle", "set up a provider dashboard", or to start the local Siren stack / connect the provider to it in dev mode. Args - dashboard=<artifact url> | dashboard=new.
---

# terraform-provider-qa: one QA cycle

One cycle = **prereqs → dashboard → scan → triage → fix (and carry to the combined branch) → local stack + regression → push → reply/resolve → dashboard → report**.
If the scan finds nothing to fix, skip fix/regression/push: refresh the dashboard and report.
To only start the stack or connect the provider to it, run the prereqs, then go to step 6.

```bash
SKD=<this skill's directory>                     # <repo>/.claude/skills/terraform-provider-qa
cfg() { python3 "$SKD/scripts/config.py" get "$1"; }
P=$(git -C "$SKD" rev-parse --show-toplevel)     # the provider repo
R=$(cfg repo); export GITHUB_USER=$(cfg github_user)
S=<scratchpad>/tfqa-<UTC timestamp>              # all work files of this cycle; mkdir -p $S/live
date -u +%Y-%m-%dT%H:%M:%SZ > $S/scan-start
```

## Config

Per-user settings and state live in `<repo>/.claude/terraform-provider-qa/config.json` (git-excluded), read and written only through `scripts/config.py` (`show`, `init`, `get <key>`, `set <key> <value>`):

| Key | Meaning |
|---|---|
| `repo` | `owner/name` on GitHub (detected) |
| `github_user` | the user's GitHub login; comments by it count as "ours" (detected) |
| `dashboard_url` | the dashboard artifact (step 0b) |
| `combined_branch` | optional branch in `$P` that carries every PR's changes; the regression runs from it. Empty = run from each PR's worktree |
| `worktree_root` | per-PR worktrees: `<worktree_root>/<PR>` |
| `siren_dir`, `minicloud_dir`, `siren_frontend_dir` | sibling repos of the local stack (default: siblings of `$P`) |
| `aws_profile` | AWS profile for minicloud's real-AWS calls (defaults to `$AWS_PROFILE`; ask the user if empty) |
| `jira_cloud_id` | Atlassian site id (ask the user, or `getAccessibleAtlassianResources`) |
| `last_scan` | ISO-8601 UTC cutoff of the last completed scan |

First run: `python3 $SKD/scripts/config.py init` detects what it can and keeps what is set. Show the result to the user and ask only for what is empty or wrong (`jira_cloud_id`, repo paths that don't exist).

## 0. Prerequisites

```bash
bash $SKD/scripts/prereq.sh               # full cycle and stack work
bash $SKD/scripts/prereq.sh --scan-only   # a cycle that only scans: gh, jq, python3
```
It checks, read-only, and prints `PASS|WARN|FAIL`:
- **gh CLI**: installed, logged in, the active account is `github_user`, it can read `repo` and run GraphQL; `jq`, `python3`.
- **siren**: exists, is a git repo, has `scripts/local/all-local.sh` and an `all-local-preflight` Make target; **siren-frontend**: git repo with `package.json`; **minicloud**: git repo with `Dockerfile`. Their branches are printed as WARN: they are used as they are and **never switched**.
- **provider repo**: `scripts/local/stack.sh` and the `build-dev`, `testacc-local`, `test-dev`, `check-release` targets exist on the checked-out branch.
- **toolchain**: `go`, `terraform`, `node`, `pnpm`, `docker` (and the daemon running), free disk.

Any FAIL: stop and tell the user exactly what to do (`gh auth login`, `gh auth switch --user …`, `git clone … <path>`, `config.py set siren_dir <path>`, start Docker Desktop). Run the scan-only variant when the cycle may not need the stack, and the full one before step 5.

## 0b. Choose the dashboard

The dashboard is a Dashboard-type artifact holding three file datasets: `prs` (`pr-status.json`, rows keyed by `pr` int), `plan` (`plan-status.json`, rows keyed by `row` string), `live` (`live-check.json`, one row). Its rows are the source of truth for which PRs this cycle checks.

Which one:
- arg `dashboard=<url>` → that artifact;
- arg `dashboard=new` → create one;
- no arg → `config.dashboard_url`; if empty, AskUserQuestion: **Use an existing dashboard** (the user pastes its URL, or picks from `Artifact list` with `type: "Dashboard"`) or **Create a new one**.

**Existing:**
1. `Artifact read` with its `url`. Continue only if the header says the user owns it or is a "writer"; otherwise stop: "this dashboard can't be changed by you; ask its owner for edit access, or create a new one".
2. `ArtifactData list` on `datasets`. Each of `prs`, `plan`, `live` must exist with `source.kind: "file"`. If any is missing, ask the user, then add it from the seeds as for a new dashboard (upload the seed, write only that dataset with `dashboard-seed.py`). If the page (`files/index.html`) is missing too, add it with `--page`.

**New:**
1. Ask the title (default "`<repo name>` QA").
2. `Artifact publish` with `type_url` from `$SKD/templates/dashboard/meta.json`, `title`, `auto_open: "after_first_write"`, nothing else. Note the returned `url`.
3. Upload each seed (`$SKD/templates/dashboard/seed/*.json`): `Artifact publish` with that `url`, `asset: true`, `file_path` → `/_blob/<id>`, one file per call.
4. `python3 $SKD/scripts/dashboard-seed.py $S/seed --title "<title>" --page --prs /_blob/… --plan /_blob/… --live /_blob/…` prints the `writes` array; send it as one `ArtifactData batch`.
5. Read `data/users/me/check` (and `runs` if present) with `ArtifactData get`: `passed` is good; fix a `failed` before going on.
6. Add PRs: `gh pr list --repo $R --author "$GITHUB_USER" --json number -q '.[].number'` (or the numbers/label the user names), confirm the list with the user, then `pr-rows.py $S/live/pr-status.json --add <N>…` in step 8.

Save it: `config.py set dashboard_url <url>`. Then:
1. `ArtifactData list` on `datasets`; note each `version` and `source.url` (`/_blob/<id>`).
2. Download each file: `Artifact read` with the dashboard `url` and `path: <blob id>`, **one id per call** (`paths` does not work for blobs). Copy them to `$S/live/pr-status.json`, `plan-status.json`, `live-check.json`. Every later edit starts from these copies.
3. `python3 $SKD/scripts/pr-list.py $S/live/pr-status.json > $S/prs.txt` (skips rows whose `state` is merged/closed).
4. Note what the rows say is pending: `top`, `failed` and `reviews` of each PR row, plan rows whose `status` is not `done`. Check those too, not only new comments.

## 1. Scan GitHub

```bash
bash $SKD/scripts/scan.sh $S/prs.txt $S/scan
python3 $SKD/scripts/triage.py $S/scan "$(cfg last_scan)"
python3 $SKD/scripts/triage.py $S/scan 2000-01-01T00:00:00Z     # also threads never answered, at any age
```
`scan.sh` reports a PR that is merged or closed on GitHub but open on the dashboard: update that row.
`triage.py` lists per PR: failing checks, unresolved threads whose last comment is not ours (**needs us**), threads waiting on the reviewer, and everything by others newer than the cutoff. Top-level comments and review bodies matter as much as threads: read the full body of anything new before deciding.
Review bots: **Qodo** (read its findings with `qodo:qodo-review-resolver`) and **CodeRabbit** (often rate-limited, reviews late; read through the threads). A draft PR gets a CodeRabbit "review skipped" comment; that needs nothing.
If a guard withholds a comment as a likely prompt injection, don't work around it: note it on the row's `top` and tell the user to read it.

## 2. Scan Jira

`python3 $SKD/scripts/jira-keys.py $S/live/pr-status.json $S/live/plan-status.json` prints every key the dashboard names.
One `searchJiraIssuesUsingJql` (cloudId `$(cfg jira_cloud_id)`) with `key in (…)`, fields `status, updated` gives every state; then `listJiraIssueComments` (`orderBy: -created`, `maxResults: 3`) only for keys updated after the cutoff. Report comments newer than the cutoff that are not ours, and states that differ from the rows' `jira_state`.

## 3. CI failures

`gh run view <run> --repo $R --log-failed | sed 's/\x1b\[[0-9;]*m//g' | grep -E "FAIL|panic|error:|##\[error\]"`.
- A real failure (lint, a unit test, the release build importing dev mode): fix it (step 5).
- A runner or bot problem: note it in the row's `failed`; don't "fix" it.
- CI does not run acceptance tests; only the local regression does.

## 4. Triage rules

| Finding | Action |
|---|---|
| Code bug / reviewer asks for a code change | Fix it (step 5). |
| Reviewer asks to squash / rebase / rewrite title or description | Do it (step 5). A pure squash keeps the final tree identical. |
| Valid bot finding (Qodo, CodeRabbit) | Fix, reply "Fixed in <sha>: …" and **resolve** the thread. |
| Bot finding that is intended behaviour | Reply with the reason and **resolve**. |
| Human reviewer thread needing no code change | Reply and **ask the reviewer to resolve**; never resolve it ourselves. |
| Reviewer disagrees with a design, or anything needing the user's decision | **Stop and ask the user** (AskUserQuestion) with a recommendation. |
| A change to `scylladbcloud_stack` | Needs agreement from the Stacks API owners (CONTRIBUTING.md): ask the user. |

Working rules:
- **One fix at a time.** After each fix: unit checks, then the local regression (step 6), then a table of all planned fixes with their status. Never batch fixes, never skip the regression for a code change.
- **Backward compatible with the latest release** (`git -C $P tag --sort=-v:refname | head -1`). Schema changes are additive; a state applied by the registry release must plan "No changes" with the local build (`dev/regress.sh` checks exactly this).
- Say "gated" for what cannot run locally (6.4), never "passed".

## 5. Fix locally

1. `git -C $P fetch -q origin`. Work on the PR that owns the code, in its worktree, from `origin/<branch>`:
   `W=$(cfg worktree_root)/<PR>`; `[ -d $W ] || git -C $P worktree add --detach $W origin/<branch>`; `git -C $W checkout -q --detach origin/<branch>`.
2. Checks in the worktree, all must pass: `make fmt && make lint && make test && make test-dev && make check-release`. A branch without dev mode has no `test-dev`/`check-release` targets: run the rest there, and all of them on the combined branch. When schema, descriptions or `templates/` changed, also `make generate`; tracked `docs/` must come out unchanged or be committed with the change. `make lint` needs `terraform` on PATH. If golangci-lint panics with "file requires newer Go version", it was built with an older Go: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version the Makefile pins>`.
3. Commit hygiene (AGENTS.md, CONTRIBUTING.md): conventional commits, subject ≤ 50 chars, imperative, body wrapped at 72, what and why; no fixup or "address review" commits left (`git commit --fixup=<sha>`, `GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash origin/<base>`); rebase on the base, never merge it in; comments only where needed.
4. Stacks: a PR's `base` on the dashboard tells what it sits on. A fix to a PR others are based on means rebasing each of them (`git rebase --onto <new base> <old base> <head>`), resolving a conflict once at the lowest commit, and checking `git diff <old head> <new head>` is only the fix.
5. If `combined_branch` is set: `git -C $P cherry-pick <sha>` on it and rerun item 2 in `$P`; the regression runs from it. Otherwise the regression runs from the PR's worktree.
6. Remove scratch worktrees when done (`git -C $P worktree remove $W`).

## 6. Local stack and regression

Start it by yourself once a code fix is ready; the only reason to wait is a busy stack (6.0). Run `prereq.sh` (full) first.

### 6.0 Before starting

```bash
bash $SKD/scripts/install-dev.sh          # installs dev/ from templates if missing; never overwrites
pgrep -fl "make all-local|all-local.sh" || echo "no all-local run"
docker ps --format '{{.Names}}\t{{.Status}}'          # siren-*, dbaas_db, minicloud_vault = a stack is up
```
- An all-local run in progress (another session's E2E) owns the stack: **stop and tell the user**; never restart or stop it.
- A stack that is up with nothing running on it (`running_vms` 0) can be reused: skip to `stack.sh up`.
- Never switch branches or edit tracked files in the siren, siren-frontend or minicloud repos. `stack.sh` only adds the dev user's mail domain to siren's generated, git-ignored `scylla-dbaas.yml.minicloud*`.

`dev/` (git-ignored) holds `full.sh`, `row.sh`, `regress.sh`, `watch.sh`, `checks/<name>/check.sh`, `local.tfrc`, `release.tfrc`, `main.tf`, `logs/`. Dev credentials: `~/.config/terraform-provider-scylladbcloud/dev-credentials.json`; token cache `~/.cache/terraform-provider-scylladbcloud/dev-token.json`; stack logs `~/.cache/terraform-provider-scylladbcloud/stack/`.

### 6.1 Start Siren on minicloud in Docker

```bash
cd $P
export GOTOOLCHAIN=auto REAL_AWS_PROFILE=$(cfg aws_profile) FETCH_GCP_IMAGES=0 \
       MINICLOUD_DOCKER=1 MINICLOUD_DIR=$(cfg minicloud_dir) SIREN_DIR=$(cfg siren_dir)
make -C "${SIREN_DIR}" all-local-preflight MINICLOUD_RESTART=1 > $S/preflight.out 2>&1   # ~4 min with a cached image
scripts/local/stack.sh up > $S/stack-up.out 2>&1 </dev/null                                # ~1 min
scripts/local/stack.sh status
```
- Laptops: keep the lid open for the whole run. `caffeinate` stops idle sleep, not a lid-close sleep, and emulated VMs that sleep through a create can hang it (Siren retries SSH forever at 30% "Creating instances"). A create stuck at 30% for over 20 minutes: stop the run, `stack.sh down`, start again, rerun what is left.
- **Send `stack.sh up` to a file, never into a pipe**: siren-frontend keeps the pipe open and the command never returns. Stopping the Bash task that ran it also kills siren-frontend; run `stack.sh up` again.
- `GOTOOLCHAIN=auto`: the host Go may be older than siren's go.mod asks for.
- `MINICLOUD_DOCKER=1` builds minicloud from its Dockerfile and runs container `siren-minicloud`. Keep the exports for every later `stack.sh` call (`stack.sh down` reads `MINICLOUD_DOCKER`).
- The preflight prints PASS/WARN/FAIL; FAIL lines must be fixed. WARN about GCP or upgrade images doesn't matter (the provider tests use AWS only). Run it in the background; a first image build takes several minutes.
- `stack.sh up` builds and starts siren-frontend (`PUBLIC_API=true`, `:3011`), creates the vector search CA in Vault (reusing an existing one), creates and verifies the dev user and billing, and checks it can log in. Its warning "siren's fake BYOK lacks the ProgrammaticAccessKeys fix" is a false alarm on current siren branches.
- `stack.sh status` must show minicloud (`:5050`), siren api (`:8080`), siren public (`:8088`), siren-frontend (`:3011`) and credentials up.
- `curl -s localhost:5050/minicloud/status`: `flags.services` `ec2_images`, `secretsmanager`, `sts` all `simulated` (`local` on older minicloud builds); `flags.guest_ca_trust` and `flags.intercept_s3` true; `running_vms` 0.
- Every start after `stack.sh down` has a **new database**: the dev user is recreated, its account id may differ, old tokens are rejected (`020006`).

### 6.2 Connect the provider in dev mode

```bash
cd $P && make build-dev        # go build -tags dev -> $(go env GOPATH)/bin
export SCYLLADB_CLOUD_DEV_MODE=1 SCYLLADB_CLOUD_ENDPOINT=http://localhost:3011
unset SCYLLADB_CLOUD_TOKEN
export TF_CLI_CONFIG_FILE=$P/dev/local.tfrc
make testacc-local run='TestAccScyllaDBCloudCluster_invalidConfig'    # plan only: proves the dev build logs in
python3 -c 'import json,os;p=os.path.expanduser("~/.cache/terraform-provider-scylladbcloud/");print(list(json.load(open(p+"dev-token.json")).values())[0]["token"],end="",file=open(p+"dev-token","w"))'
curl -s -m10 http://localhost:3011/account/default -H "Authorization: Bearer $(cat ~/.cache/terraform-provider-scylladbcloud/dev-token)"   # {"data":{"accountId":…
```
Do this after every stack start; the token lasts one day. The last two lines give `dev/regress.sh`'s release phase (the registry release has no dev mode) the token the dev build cached.
- Dev mode exists only in the `-tags dev` build; `make check-release` proves the release build has none.
- With `SCYLLADB_CLOUD_DEV_MODE=1` and no token the provider logs in to Siren itself (`http://127.0.0.1:8080/api/v1`, override with `SCYLLADB_CLOUD_DEV_SIREN_ENDPOINT`) with the dev credentials file. Loopback endpoints only.
- `SCYLLADB_CLOUD_ENDPOINT` is siren-frontend's public API: camelCase bodies, unknown names silently dropped. Only a live run catches that.
- Acceptance tests: `make testacc-local run='<regex>'`. By hand: `terraform plan/apply` in `$P/dev`. Quick end-to-end proof with no cluster: `dev/checks/emails/check.sh` (~10 s, three PASS lines).

### 6.3 What to run

| What changed | Run |
|---|---|
| Docs, comments, examples, CHANGELOG | unit checks only, no stack |
| Validation or plan-time logic | + `make testacc-local run='TestAccScyllaDBCloudCluster_invalidConfig'` |
| Any resource, data source or client code | `dev/full.sh`, about 1–2 h |
| A resource with its own check | its `dev/checks/<name>/check.sh` first, then `dev/full.sh` |

`dev/full.sh` = stack check → `make testacc-local` → `dev/row.sh` (unit, `dev/regress.sh all` with the release build, then the dev build) → every `dev/checks/*/check.sh`. Logs: `dev/logs/<timestamp>/<step>.log`.

```bash
cd $P && caffeinate -dims dev/full.sh > $S/full.out 2>&1        # Bash run_in_background (caffeinate: macOS only)
```
Watch it with a Monitor that has a deadline and reports FAIL when it passes (wrap in `bash -c`):
```bash
bash -c 'F='"$S"'/full.out; end=$(( $(date +%s) + 10800 )); prev=""
while [ "$(date +%s)" -lt "$end" ]; do
  phase=$(grep "^### " $F | tail -1); step=$(grep "^== " $F | tail -1 | cut -c1-80)
  vms=$(curl -s -m3 localhost:5050/minicloud/status | python3 -c "import json,sys;print(json.load(sys.stdin)[\"running_vms\"])" 2>/dev/null)
  req=$(docker exec dbaas_db mysql -usiren -p101202 scylla_dbaas -N -e "select concat(id,\" \",request_type,\" \",status,\" \",progress_percent,\"% \",progress_description) from cluster_request where request_type not like \"%CHECK%\" order by id desc limit 1" 2>/dev/null)
  cur="$phase | $step | vms=${vms:-?} | $req"
  [ "$cur" != "$prev" ] && { echo "$(date +%H:%M:%S) $cur"; prev="$cur"; }
  grep -qE "^### ALL PASS$|^FAIL |^UNIT FAIL$|^STACK DOWN$|^--- FAIL" $F && { echo "DONE: $(grep -E "^(FAIL|### ALL PASS|UNIT FAIL|STACK DOWN|--- FAIL)" $F | tr "\n" ";")"; exit 0; }
  sleep 30
done; echo "FAIL: no result after 3h, last: $prev"; exit 1'
```
- **Expected:** `### ALL PASS`. A cluster create is silent for about 17 minutes; that is normal.
- On a failure read `dev/logs/<latest>/<step>.log`, then siren's side: `docker logs siren-scylla-cloud-bootstrap-1 2>&1 | grep "Failed bootstrap request"` and the `cluster_request` row.
- A real failure: fix it (step 5), rerun. Never push before the regression passes.
- After a killed run: `SCYLLADB_CLOUD_DEV_MODE=1 make sweep tags=dev` (clusters `tf-acc-test-*`), and `terraform destroy` in `dev/`.

### 6.4 Known traps

- **Stale service locks** after a Siren restart while workers run: stop the workers, `docker exec siren-scylla-cloud-api-1 scylla-cloud service lock release --service-lock-id N --config /scylla-dbaas.yml`, restart them.
- **Vector search CA**: `all-local-down` wipes Vault, and INSTALL_VECTOR_SEARCH fails at 30% without the CA. `stack.sh up` creates it; if its output has no `==> vector search CA` line (an older `stack.sh`), run `docker exec siren-scylla-cloud-api-1 scylla-cloud cluster vectorstore generate-ca --config /scylla-dbaas.yml` (idempotent) before a vector search test.
- **Missing AMIs**: `AWS_PROFILE=$(cfg aws_profile) minicloud --cache-dir ~/.cache/minicloud cache-image aws us-east-1/ami-…` (or `FETCH_AMIS=1` on the preflight).
- **No KVM in Docker on macOS**: a 3-node cluster takes ~17 min, a vector install ~13 min. minicloud fits one cluster at a time; a stuck delete starves the next create: `stack.sh down`, start again.
- **Dev user cannot log in**: delete `dev-credentials.json`, run `stack.sh up` again.
- **Release phase fails with `020006`**: the plain `dev-token` is from an older database; refresh it (6.2).
- **Account id** changes with the database: local checks read it from `/account/default`, never hardcode it.
- **Gated locally** (say so, not "passed"): encryption at rest (fake BYOK has no KMS config; runs use `enabled = false`), VPC peering and cluster connections (no real peer VPC or transit gateway), the `cql_auth` data source (`/cluster/connect` is not public).
- **Network outages are not test failures** (okta/DNS lookups failing, `downloads.scylladb.com` 500): rerun the step.

### 6.5 Stop

With the exports of 6.1: `scripts/local/stack.sh down > $S/stack-down.out 2>&1 </dev/null`. Stop the stack when the cycle ends unless the user asked to keep it.
`stack.sh down` says "stopped siren-frontend" but **leaves it running**. Stop the listener and check:
```bash
pid=$(lsof -nP -iTCP:3011 -sTCP:LISTEN | awk 'NR==2{print $2}')
[ -n "${pid}" ] && { kill "${pid}"; sleep 5; kill -9 "${pid}" 2>/dev/null; }
docker ps --format '{{.Names}}'                  # no siren-*, dbaas_db, minicloud_vault
pgrep -fl "build/server.js|byokfake|all-local.sh" || echo clean
```
Stop a background run you started with `TaskStop`. If a kill is refused, give the user the command.

## 7. Push, reply, resolve

- **Fast-forward pushes**: push directly (`git -C $W push origin HEAD:refs/heads/<branch>`; `git -C $P push origin $(cfg combined_branch)`).
- **Force-pushes** (rebases, squashes, restacks) are for the user to run. Write `$S/push.sh` and ask them to run `! bash $S/push.sh`:
  - `--force-with-lease=refs/heads/<branch>:<full old sha>`, each sha checked with `git rev-parse --verify` and against `origin/<branch>` right before handing over;
  - write `${var}` (zsh reads `$var:r` as a modifier); `bash -n` the script;
  - push base PRs before the PRs stacked on them;
  - each push's `gh pr edit` / replies go under that push's success check; always `--body-file`;
  - a `step()` helper that prints OK/FAILED and continues.
- Thread reply + resolve (one thread per call):
  ```bash
  gh api graphql -f query='mutation($t:ID!,$b:String!){addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId:$t,body:$b}){comment{url}}}' -f t=<PRRT_id> -f b="<text>"
  gh api graphql -f query='mutation($t:ID!){resolveReviewThread(input:{threadId:$t}){thread{isResolved}}}' -f t=<PRRT_id>
  ```
  Resolve only threads fixed in code, and bot threads.
- Top-level requests get a top-level answer: `gh pr comment N --repo $R --body-file <file>`.
- Draft ↔ ready (`gh pr ready N --repo $R [--undo]`) only when the user asks.
- PR titles and bodies: plain sentences about what changes for the provider's users and why; follow the PR template; keep `Closing CLOUD-xxxx` / `Fixing CLOUD-xxxx` lines.

## 8. Dashboard

Other sessions may write the same dashboard. **Read before writing, and merge; never overwrite.**
1. Start from step 0b's copies. If much time passed, re-list `datasets`: same `version` = copy is current; else download again and redo the edits.
2. `prs`: `python3 $SKD/scripts/pr-rows.py $S/live/pr-status.json [--add N …]` refreshes what GitHub knows (`title`, `url`, `state`, `base`, `commit`, `ci`, `checks`, `failed`, sizes, `checked`) and keeps the rest. Then by hand: `reviews` (e.g. `Qodo: 2 medium · CodeRabbit: 1 minor`), `top` (the one finding that matters most), `rows` (plan rows the PR carries), `jira` (`Closing CLOUD-…`), `jira_state`.
3. `plan`: for each plan row touched, `unit`, `regression` (what ran, result), `note` (commit sha), `status`, `jira_state`. A fix not in the plan gets a new row (`row`, `step`, `area`, `status`, `unit`, `regression`, `note`, `prs`, `jira`).
4. `live`: one row `{checked, phase, step, request, vms}`; set it when the regression starts, when its phase changes, and back to `{"phase": "Idle: no regression running", …}` at the end.
5. Upload each changed file: `Artifact publish` with the dashboard `url`, `asset: true`, `file_path` → new `/_blob/<id>`.
6. `ArtifactData batch`, `op: update` on `datasets/<id>`, pinned with `if_version`:
   `{"source": {"kind": "file", "format": "json", "name": "<file name>", "url": "/_blob/<id>"}, "updated": {"at": "<UTC now>", "by": "Claude"}, "edit": {"via": "claude", "at": "<UTC now>"}}`.
   Write the **whole `source` object**; a top-level `url` is silently ignored. On a version mismatch: re-read, merge, retry.
7. Read `datasets` back and check each `source.url` is the new blob.

## 9. Finish

- `python3 $SKD/scripts/config.py set last_scan "$(cat $S/scan-start)"`, only after the cycle completes, so an aborted cycle rescans.
- Stop the stack (6.5), remove scratch worktrees, check free disk.
- Tell the user what only they can run: force-push scripts, restarting Docker Desktop, killing processes.
- Report, short and plain: what was new on GitHub, Jira and CI; what was fixed (shas) and the table of planned fixes with their status; the regression result (steps run, what is gated); what was pushed and what the user must run; what still waits on others.
