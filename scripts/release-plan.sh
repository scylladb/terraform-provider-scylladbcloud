#!/usr/bin/env bash
# Plans a release of the commit checked out: the next version, the release
# notes and the checks. Builds nothing. The release workflow runs it in its
# plan job; `make release-preview` runs it locally. See RELEASING.md.
#
# Environment:
#   BUMP     auto (default), patch, minor or major
#   SUMMARY  optional paragraph at the top of the release notes
#   OUT      directory for the results (default: a new temporary directory)
#
# Results in OUT: plan.env (last, version, sha, level), notes.md,
# warnings.md and preview.md. Exits 1 if there is nothing to release or a
# check fails.
set -euo pipefail

BUMP="${BUMP:-auto}"
SUMMARY="${SUMMARY:-}"
OUT="${OUT:-$(mktemp -d)}"

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
last="$(go tool svu current)"
if [ "$BUMP" = auto ]; then
	version="$(go tool svu next)"
else
	version="$(go tool svu "$BUMP")"
fi
if [ "$version" = "$last" ]; then
	fail "No feat or fix PR since ${last}, so there is nothing to release. Choose a bump to release anyway."
fi
if git rev-parse -q --verify "refs/tags/${version}" >/dev/null ||
	git ls-remote --exit-code --tags origin "refs/tags/${version}" >/dev/null 2>&1; then
	fail "Tag ${version} already exists."
fi

IFS=. read -r lmaj lmin _ <<<"${last#v}"
IFS=. read -r nmaj nmin _ <<<"${version#v}"
if [ "$nmaj" != "$lmaj" ]; then
	level="major"
elif [ "$nmin" != "$lmin" ]; then
	level="minor"
else
	level="patch"
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
{
	if [ -n "$SUMMARY" ]; then printf '%s\n\n' "$SUMMARY"; fi
	sed '/./,$!d' "$OUT/entries.md"
} >"$OUT/notes.md"

# Checks of the bump against the entries.
# shellcheck disable=SC2086 # entry_files is a list of paths without spaces
types="$(sed -n 's/^```release-note:\(.*\)$/\1/p' $entry_files /dev/null | sort -u)"
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
	echo "| Commit | \`${sha}\` |"
	echo
	if [ -s "$OUT/warnings.md" ]; then
		echo "### Warnings"
		echo
		cat "$OUT/warnings.md"
		echo
	fi
	echo "### Release notes"
	echo
	cat "$OUT/notes.md"
	echo
	echo "<details><summary>Commits since ${last}</summary>"
	echo
	# shellcheck disable=SC2016 # literal backticks for Markdown
	git log --format='- `%h` %s' "${last}..HEAD"
	echo
	echo "</details>"
} >"$OUT/preview.md"

if [ -z "${GITHUB_ACTIONS:-}" ]; then
	cat "$OUT/preview.md"
	echo
	echo "Results in ${OUT}"
fi
