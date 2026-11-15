#!/usr/bin/env bash
# Usage: prereq.sh [--scan-only]
# Read-only checks before a cycle. One "PASS|WARN|FAIL <check>: <detail>" line each;
# exits 1 if anything FAILs. --scan-only checks only what a GitHub/Jira scan needs.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
cfg() { python3 "$here/config.py" get "$1"; }
scan_only=0; [[ "${1:-}" == --scan-only ]] && scan_only=1
fails=0
pass() { echo "PASS $1: $2"; }
warn() { echo "WARN $1: $2"; }
fail() { echo "FAIL $1: $2"; fails=$((fails + 1)); }
have() { command -v "$1" >/dev/null 2>&1; }

repo=$(cfg repo); user=$(cfg github_user)
[[ -n "$(cfg aws_profile)" ]] || warn aws_profile "empty; minicloud needs a real AWS profile for images (config.py set aws_profile <profile>)"
root=$(git -C "$here" rev-parse --show-toplevel)

# gh CLI
if ! have gh; then
	fail gh "not installed (brew install gh)"
else
	pass gh "$(gh --version | head -1)"
	if gh auth status >/dev/null 2>&1; then pass gh-auth "logged in"; else fail gh-auth "not logged in (gh auth login)"; fi
	[[ -z "${GH_TOKEN:-}" && -n "$user" ]] && GH_TOKEN=$(gh auth token --user "$user" 2>/dev/null) && export GH_TOKEN
	login=$(gh api user -q .login 2>/dev/null)
	if [[ -z "$login" ]]; then fail gh-user "gh api user failed"
	elif [[ -n "$user" && "$login" != "$user" ]]; then fail gh-user "active account is $login, config github_user is $user (gh auth switch --user $user)"
	else pass gh-user "$login"; fi
	if [[ -z "$repo" ]]; then fail gh-repo "config repo is empty (config.py init)"
	elif gh repo view "$repo" --json name >/dev/null 2>&1; then pass gh-repo "$repo readable"
	else fail gh-repo "cannot read $repo as ${login:-?}"; fi
	if gh api graphql -f query='{viewer{login}}' >/dev/null 2>&1; then pass gh-graphql ok; else fail gh-graphql "GraphQL call failed"; fi
fi
have jq && pass jq "$(jq --version)" || fail jq "not installed (scan.sh needs it)"
have python3 && pass python3 "$(python3 --version 2>&1)" || fail python3 "not installed"

if ((scan_only)); then
	((fails)) && exit 1 || exit 0
fi

branch() { git -C "$1" branch --show-current 2>/dev/null || echo "?"; }
repo_dir() { # name dir required-files...
	local name=$1 dir=$2; shift 2
	if [[ -z "$dir" || ! -d "$dir" ]]; then fail "$name" "missing at '${dir}' (clone it there, or config.py set ${name//-/_}_dir <path>)"; return 1; fi
	if ! git -C "$dir" rev-parse --git-dir >/dev/null 2>&1; then fail "$name" "$dir is not a git repo"; return 1; fi
	local f
	for f in "$@"; do [[ -e "$dir/$f" ]] || { fail "$name" "$dir has no $f"; return 1; }; done
	pass "$name" "$dir"
	warn "$name-branch" "on $(branch "$dir") (used as it is, never switched)"
}

if repo_dir siren "$(cfg siren_dir)" scripts/local/all-local.sh Makefile; then
	grep -q '^all-local-preflight' "$(cfg siren_dir)/Makefile" || fail siren "Makefile has no all-local-preflight target"
fi
repo_dir siren-frontend "$(cfg siren_frontend_dir)" package.json
repo_dir minicloud "$(cfg minicloud_dir)" Dockerfile

# the provider's own local-stack support
[[ -x "$root/scripts/local/stack.sh" ]] && pass stack.sh "present" || fail stack.sh "scripts/local/stack.sh not on branch $(branch "$root")"
for t in build-dev testacc-local test-dev check-release; do
	grep -q "^$t:" "$root/Makefile" || fail make "no '$t' target on branch $(branch "$root")"
done

for t in go terraform node pnpm docker; do
	have "$t" && pass "$t" "$(command -v "$t")" || fail "$t" "not on PATH"
done
if have docker; then docker info >/dev/null 2>&1 && pass docker-daemon running || fail docker-daemon "not running (start Docker Desktop)"; fi
vol=/System/Volumes/Data; [[ -d $vol ]] || vol=/
free=$(df -g "$vol" 2>/dev/null | awk 'NR==2{print $4}')
[[ -n "$free" ]] && { ((free < 40)) && warn disk "${free} GB free (the minicloud image and VMs need tens of GB)" || pass disk "${free} GB free"; }

((fails)) && exit 1 || exit 0
