#!/usr/bin/env bash
# Plans a release of the commit checked out: the next version, the release
# notes and the checks. Builds nothing. The release workflow runs it in its
# plan job; `make release-preview` runs it locally. See RELEASING.md.
#
# Environment:
#   BUMP     auto (default), patch, minor or major
#   SUMMARY  optional paragraph at the top of the release notes
#   OUT      directory for the results (default: a new temporary directory)
#   REPO     owner/name on GitHub (default: GITHUB_REPOSITORY, or origin)
#
# PR authors are read with the gh CLI. Without it, the list has no authors.
#
# Results in OUT: plan.env (last, version, sha, level), notes.md,
# warnings.md and preview.md. Exits 1 if there is nothing to release or a
# check fails.
set -euo pipefail

BUMP="${BUMP:-auto}"
SUMMARY="${SUMMARY:-}"
OUT="${OUT:-$(mktemp -d)}"
REPO="${REPO:-${GITHUB_REPOSITORY:-}}"

fail() {
	if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::error::$1"; else echo "error: $1" >&2; fi
	exit 1
}

cd "$(git rev-parse --show-toplevel)"
mkdir -p "$OUT"

if [ "$(git rev-parse --is-shallow-repository)" = true ]; then
	fail "The clone is shallow. Run: git fetch --unshallow --tags"
fi
case "$BUMP" in auto | patch | minor | major) ;; *) fail "BUMP must be auto, patch, minor or major, not \"$BUMP\"." ;; esac

sha="$(git rev-parse HEAD)"
last="$(git tag --merged HEAD --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1 || true)"
if [ -z "$last" ]; then
	fail "No vX.Y.Z tag before HEAD."
fi

# The bump of each commit comes from its subject, which is the PR title:
# "type!:" or a "BREAKING CHANGE:" footer line gives major, "feat" minor,
# "fix" patch. Only the start of the subject and the start of footer lines
# count, so a "fix:" quoted inside a title or a body does not.
bump_of() {
	local subject body
	subject="$(git log -1 --format=%s "$1")"
	body="$(git log -1 --format=%b "$1")"
	if grep -qE '^[a-z]+(\([^)]*\))?!: ' <<<"$subject" || grep -qE '^BREAKING[ -]CHANGE: ' <<<"$body"; then
		echo 3
	elif grep -qE '^feat(\([^)]*\))?: ' <<<"$subject"; then
		echo 2
	elif grep -qE '^fix(\([^)]*\))?: ' <<<"$subject"; then
		echo 1
	else
		echo 0
	fi
}
case "$BUMP" in
patch) rank=1 ;;
minor) rank=2 ;;
major) rank=3 ;;
auto)
	rank=0
	for commit in $(git rev-list "${last}..HEAD"); do
		r="$(bump_of "$commit")"
		if [ "$r" -gt "$rank" ]; then rank="$r"; fi
	done
	;;
esac
if [ "$rank" -eq 0 ]; then
	fail "No feat or fix PR since ${last}, so there is nothing to release. Choose a bump to release anyway."
fi

IFS=. read -r major minor patch <<<"${last#v}"
case "$rank" in
3) level="major" version="v$((major + 1)).0.0" ;;
2) level="minor" version="v${major}.$((minor + 1)).0" ;;
1) level="patch" version="v${major}.${minor}.$((patch + 1))" ;;
esac
if git rev-parse -q --verify "refs/tags/${version}" >/dev/null ||
	git ls-remote --exit-code --tags origin "refs/tags/${version}" >/dev/null 2>&1; then
	fail "Tag ${version} already exists."
fi

# Release notes. When the last tag has no .changelog/ yet, "-" takes every
# entry in the directory.
if git cat-file -e "${last}:.changelog" 2>/dev/null; then
	from="$last"
	entry_files="$(git diff --name-only --diff-filter=A "$last" HEAD -- .changelog/)"
else
	from=-
	entry_files="$(git ls-files .changelog/)"
fi
go tool changelog-build -git-dir "$PWD" -last-release "$from" -this-release "$sha" \
	-entries-dir .changelog \
	-changelog-template .github/changelog/release-notes.tmpl \
	-note-template .github/changelog/note.tmpl >"$OUT/entries.md"
# Prints the result of a GitHub API query, or nothing if it fails.
gh_get() {
	local result
	if result="$(gh api "$1" --jq "$2" 2>/dev/null)"; then printf '%s' "$result"; fi
}

# Every change since the last tag, with the PR authors. The @mentions make
# GitHub show the contributors of the release.
if [ -z "$REPO" ]; then
	REPO="$(git remote get-url origin | sed -E 's#^(https://github\.com/|git@github\.com:)##; s#\.git$##')"
fi
: >"$OUT/changes.md"
while read -r short subject; do
	pr="$(sed -n 's/.*(#\([0-9]\{1,\}\))$/\1/p' <<<"$subject")"
	author=""
	if [ -n "$pr" ]; then
		author="$(gh_get "repos/${REPO}/pulls/${pr}" .user.login)"
	else
		# Commits merged before squash merges have no "(#N)" in the subject.
		found="$(gh_get "repos/${REPO}/commits/${short}/pulls" '.[0] | "\(.number) \(.user.login)"')"
		if [ -n "$found" ] && [ "$found" != "null null" ]; then read -r pr author <<<"$found"; fi
	fi
	if [ -n "$pr" ]; then
		line="* ${subject% (#"$pr")}"
		if [ -n "$author" ]; then line="${line} by @${author}"; fi
		echo "${line} in #${pr}" >>"$OUT/changes.md"
	else
		echo "* ${subject} (${short})" >>"$OUT/changes.md"
	fi
done < <(git log --reverse --format='%h %s' "${last}..HEAD")

{
	if [ -n "$SUMMARY" ]; then printf '%s\n\n' "$SUMMARY"; fi
	sed '/./,$!d' "$OUT/entries.md"
	echo
	echo "<details><summary>All changes ($(wc -l <"$OUT/changes.md" | tr -d ' '))</summary>"
	echo
	cat "$OUT/changes.md"
	echo
	echo "</details>"
	echo
	echo "**Full changelog**: https://github.com/${REPO}/compare/${last}...${version}"
} >"$OUT/notes.md"

# Checks of the bump against the entries.
# The entries are read from the planned commit, as changelog-build reads
# them, not from the working tree.
types="$(for f in $entry_files; do git show "${sha}:${f}"; done | tr -d '\r' | sed -n 's/^```release-note:\(.*\)$/\1/p' | sort -u)"
if grep -qx breaking-change <<<"$types" && [ "$level" != major ]; then
	fail "A breaking-change entry needs a major release, but ${version} is a ${level} release."
fi
: >"$OUT/warnings.md"
if grep -qxE 'feature|new-resource|new-data-source' <<<"$types" && [ "$level" = patch ]; then
	echo "- A feature entry is released in a patch version." >>"$OUT/warnings.md"
fi
git log --format='%s' "${last}..HEAD" | while read -r subject; do
	pr="$(sed -n 's/.*(#\([0-9]\{1,\}\))$/\1/p' <<<"$subject")"
	if grep -qE '^(feat|fix)(\(.*\))?!?: ' <<<"$subject" && [ -n "$pr" ] && [ ! -f ".changelog/${pr}.txt" ]; then
		echo "- \`${subject}\` has no changelog entry." >>"$OUT/warnings.md"
	fi
done
if ! grep -q '[^[:space:]]' "$OUT/entries.md"; then
	echo "- The release notes have no entries." >>"$OUT/warnings.md"
fi

{
	echo "last=${last}"
	echo "version=${version}"
	echo "sha=${sha}"
	echo "level=${level}"
} >"$OUT/plan.env"

{
	echo "## Release preview: ${version}"
	echo
	echo "| | |"
	echo "|---|---|"
	echo "| Version | ${last} → **${version}** (${level}, bump: ${BUMP}) |"
	echo "| Release title | ${version} |"
	echo "| Commit | \`${sha}\` |"
	echo
	if [ -s "$OUT/warnings.md" ]; then
		echo "### Warnings"
		echo
		cat "$OUT/warnings.md"
		echo
	fi
	echo "### Release body"
	echo
	echo "Everything in the quote below is published as the body of the release."
	echo
	sed 's/^/> /' "$OUT/notes.md"
} >"$OUT/preview.md"

if [ -z "${GITHUB_ACTIONS:-}" ]; then
	cat "$OUT/preview.md"
	echo
	echo "Results in ${OUT}"
fi
