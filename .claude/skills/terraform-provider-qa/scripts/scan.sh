#!/usr/bin/env bash
# Usage: scan.sh <prs.txt> <out-dir>
# prs.txt: "<owner>/<repo> <number>" per line (from pr-list.py, i.e. the dashboards).
# Writes <out-dir>/<repo>-<N>.json: state, review threads, top-level comments, reviews,
# last-commit CI rollup. PRs that are merged/closed on GitHub are reported and their files removed.
set -uo pipefail
list=${1:?prs.txt}; out=${2:?out dir}; mkdir -p "$out"
# With several gh accounts, GITHUB_USER picks the one that can read the repo.
if [[ -z "${GH_TOKEN:-}" && -n "${GITHUB_USER:-}" ]]; then
  GH_TOKEN=$(command gh auth token --user "$GITHUB_USER" 2>/dev/null) && export GH_TOKEN
fi
Q='query($o:String!,$r:String!,$n:Int!){repository(owner:$o,name:$r){pullRequest(number:$n){number title state isDraft reviewDecision mergeable headRefName headRefOid
 reviewThreads(first:100){nodes{id isResolved path line comments(first:50){nodes{author{login} createdAt body}}}}
 comments(last:30){nodes{author{login} createdAt body}}
 reviews(last:30){nodes{author{login} state submittedAt body}}
 commits(last:1){nodes{commit{committedDate statusCheckRollup{state contexts(first:60){nodes{... on CheckRun{name conclusion status detailsUrl} ... on StatusContext{context state targetUrl}}}}}}}}}}'
while read -r repo n; do
  [[ -z "$repo" ]] && continue
  o=${repo%/*}; r=${repo#*/}
  gh api graphql -f query="$Q" -f o="$o" -f r="$r" -F n="$n" > "$out/$r-$n.json" &
done < "$list"
wait
for f in "$out"/*.json; do
  s=$(jq -r '.data.repository.pullRequest.state // "ERROR"' "$f")
  if [[ "$s" != OPEN ]]; then echo "skip $(basename "$f" .json): $s on GitHub (update the dashboard row)"; rm -f "$f"; fi
done
echo "scanned $(ls "$out"/*.json | wc -l | tr -d ' ') open PRs"
