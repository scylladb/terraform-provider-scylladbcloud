#!/usr/bin/env python3
"""Usage: pr-list.py <dashboard-json>... > prs.txt
Reads the PR rows of the dashboard dataset (pr-status.json)
and prints "<owner>/<repo> <number>" for every PR not Merged/Closed (state from the
dashboard; scan.sh re-checks the live state and skips PRs closed since)."""
import json, re, sys
seen = []
for p in sys.argv[1:]:
    d = json.load(open(p)); rows = d["rows"] if isinstance(d, dict) else d
    for r in rows:
        if str(r.get("state", "")).lower() in ("merged", "closed"):
            continue
        m = re.search(r"github\.com/([^/]+/[^/]+)/pull/(\d+)", r.get("url") or "")
        if not m:
            print(f"!! no PR url in row: {r.get('pr') or r.get('number')}", file=sys.stderr); continue
        key = f"{m.group(1)} {m.group(2)}"
        if key not in seen:
            seen.append(key)
print("\n".join(seen))
