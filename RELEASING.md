# Releasing

A release publishes a signed build of the provider as a GitHub release.
The [Terraform Registry](https://registry.terraform.io/providers/scylladb/scylladbcloud)
picks it up from there. A published version cannot be changed, so the
release workflow shows a preview and waits for a second release owner to
approve it.

## Rules that cannot be undone

- **Never move, delete or push again a published tag.** Users' lock files
  hold the checksums of each version. To fix a bad release, release the
  next patch version.
- **Never replace the files of a published release.** Editing the release
  notes is safe: neither the Registry nor Terraform reads them.
- **Never reuse a version number**, even after deleting a release.

## Where the version and the notes come from

PRs are squash-merged, so each PR is one commit on `main` with the PR
title as its subject (see [CONTRIBUTING.md](CONTRIBUTING.md)).

- **Version:** [svu](https://github.com/caarlos0/svu) reads the commit
  subjects since the last tag. `feat` gives a minor version, `fix` a patch
  version, and `!` before the colon or a `BREAKING CHANGE:` footer a major
  version. Other types give no new version.
- **Release notes:** [go-changelog](https://github.com/hashicorp/go-changelog)
  collects the `.changelog/<PR number>.txt` files added since the last tag.
  The templates are in `.github/changelog/`. Do not add other files to
  `.changelog/`: go-changelog reads every new file there as an entry.

## How to release

1. Make sure everything for the release is merged to `main`.
2. Optionally preview it locally: `make release-preview` (see below).
3. Go to **Actions → release → Run workflow**, choose branch `main`, and
   set:
   - **bump:** `auto`, or `patch`, `minor` or `major` to override it.
   - **summary:** an optional paragraph at the top of the release notes.
   - **Dry Run:** tick it to see the preview and the build without
     publishing.
4. The workflow runs three jobs:

   | Job | Takes | Does |
   |---|---|---|
   | `plan` | about 1 minute | Picks the version, builds the release notes, runs the checks and writes the preview on the run page. |
   | `build` | about 6 minutes | Builds and signs the 16 release files with GoReleaser, without publishing them, and adds the signing key and checksums to the preview. |
   | `publish` | after approval | Waits in the `release` environment. On approval, creates the tag and the release from exactly the files `build` made, then waits until the Registry lists the version. Skipped in a dry run. |

5. Read the preview on the run page. If something is wrong, cancel the run
   or reject the `publish` job. Nothing is published, and no tag is
   created.
6. The other release owner opens **Review deployments** on the run page and
   approves. The person who started the run cannot approve it.

The release uses the commit that `plan` saw. Merges to `main` after the
run started are not included.

## The checks

| Result | When |
|---|---|
| **Stops** | No `feat` or `fix` PR since the last tag and bump is `auto`: nothing to release. Choose a bump to release anyway. |
| **Stops** | The tag already exists. |
| **Stops** | A `breaking-change` entry, and the version is not a major version. |
| **Stops** | The run was not started from `main`. |
| **Warns** | A `feature`, `new-resource` or `new-data-source` entry in a patch version. |
| **Warns** | A `feat` or `fix` PR without a changelog entry. |
| **Warns** | The release notes have no entries. |

A warning does not stop the release. Read it before you approve.

## Local preview

`make release-preview` runs the same script as the `plan` job,
[`scripts/release-plan.sh`](scripts/release-plan.sh), and prints the
preview. It builds nothing and takes a few seconds.

```bash
make release-preview
make release-preview BUMP=patch SUMMARY="Maintenance release."
```

It needs the full git history and the tags. In a shallow clone, run
`git fetch --unshallow --tags` first. It reads commits, not uncommitted
changes.

To see how merges would change a release, commit on a throwaway branch
and preview it:

```bash
git switch -c release-try
printf '%s\n' '```release-note:bug' 'resource/scylladbcloud_cluster: Fix something' '```' > .changelog/9001.txt
git add .changelog/9001.txt
git commit -m "fix(cluster): fix something (#9001)"
make release-preview
git switch main && git branch -D release-try
```

## Setup

- **`release` environment** (Settings → Environments):
  - Required reviewers: the release owners.
  - Prevent self-review: on.
  - Allow administrators to bypass: off.
  - Deployment branches: `main` only.
- **Repository secrets:** `GPG_PRIVATE_KEY` and `PASSPHRASE`. The `build`
  job signs with them before approval, so they are not environment secrets.
- **Terraform Registry:** the public half of the GPG key is in the
  settings of the `scylladb` namespace in the Registry.

## Testing changes to the release workflow

Test in a personal fork, never in this repository.

1. Fork the repository. GitHub lists the fork's workflows after the first
   push to it.
2. Create a throwaway GPG key (RSA 4096, short expiry) in a separate
   keyring, and add it as the fork's `GPG_PRIVATE_KEY` and `PASSPHRASE`
   secrets.
3. Create the `release` environment in the fork with yourself as the
   reviewer and Prevent self-review off.
4. Push the change to the fork's `main`: **Run workflow** lists only
   workflows on the default branch.
5. Test the logic of `scripts/release-plan.sh` locally first. Use the fork
   for the build, the approval and the publish.

The fork skips the Registry step. Delete the fork's test releases and tags
afterwards.

## Troubleshooting

| Problem | Fix |
|---|---|
| "No feat or fix PR since …" | Nothing user-facing was merged, or the PR titles have the wrong type. Choose a bump to release anyway. |
| "A breaking-change entry needs a major release" | Release as `major`, or change the entry if the change is not breaking. |
| "… does not match its checksum in SHA256SUMS" | The signed checksums do not list `terraform-registry-manifest.json`. Check `checksum.extra_files` in `.goreleaser.yml`. |
| The Registry does not list the version after 10 minutes | The Registry did not get or accept the release. The namespace owner can resync the provider in the Registry. Check that the release has all 16 files and that the signing key matches the key in the Registry. |
| A published release is wrong | Release the next patch version. Never change the tag or the files. |

## Rotating the signing key

The private key is a repository secret, and its public half is in the
Registry. Change both together: a release signed with a key the Registry
does not know is rejected. Only the Registry namespace owner can change
the public key.
