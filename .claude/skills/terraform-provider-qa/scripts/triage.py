#!/usr/bin/env python3
"""Usage: triage.py <scan-dir> <cutoff-ISO8601Z>
Prints, per PR: CI failures, unresolved threads whose last comment is NOT ours,
unresolved threads waiting on the reviewer, and every comment / review / thread
comment by someone else newer than the cutoff. "Us" is $GITHUB_USER, else the gh login."""
import json, glob, sys, os, subprocess
ME = os.environ.get("GITHUB_USER") or subprocess.run(["gh", "api", "user", "-q", ".login"], capture_output=True, text=True).stdout.strip()
d, cut = sys.argv[1], sys.argv[2]
BAD = ("FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED")
for f in sorted(glob.glob(os.path.join(d, "*.json"))):
    try:
        pr = json.load(open(f))["data"]["repository"]["pullRequest"]
    except Exception as e:
        print(f"!! {f}: {e}"); continue
    name = os.path.basename(f)[:-5]
    roll = pr["commits"]["nodes"][0]["commit"]["statusCheckRollup"] or {}
    fails = [c.get("name") or c.get("context") for c in (roll.get("contexts") or {}).get("nodes", [])
             if (c.get("conclusion") or c.get("state")) in BAD]
    threads = pr["reviewThreads"]["nodes"]
    unres_them = [t for t in threads if not t["isResolved"] and t["comments"]["nodes"][-1]["author"]["login"] != ME]
    unres_us = [t for t in threads if not t["isResolved"] and t["comments"]["nodes"][-1]["author"]["login"] == ME]
    new = []
    for c in pr["comments"]["nodes"]:
        if c["createdAt"] > cut and c["author"]["login"] != ME:
            new.append(("COMMENT", c["author"]["login"], c["createdAt"], "", c["body"]))
    for r in pr["reviews"]["nodes"]:
        if (r["submittedAt"] or "") > cut and r["author"]["login"] != ME and r["body"].strip():
            new.append((f"REVIEW {r['state']}", r["author"]["login"], r["submittedAt"], "", r["body"]))
    for t in threads:
        for c in t["comments"]["nodes"]:
            if c["createdAt"] > cut and c["author"]["login"] != ME:
                st = "resolved" if t["isResolved"] else "OPEN"
                new.append((f"THREAD {st}", c["author"]["login"], c["createdAt"], f"{t['id']} {t['path']}", c["body"]))
    if not (fails or unres_them or new):
        continue
    print(f"=== {name} [{pr.get('headRefName','?')}] {pr['reviewDecision']} mergeable={pr.get('mergeable','?')} draft={pr['isDraft']}")
    if fails: print("  CI failing:", ", ".join(fails))
    for t in unres_them: print(f"  UNRESOLVED (needs us): {t['id']} {t['path']}:{t['line']}")
    if unres_us: print(f"  waiting on reviewer: {len(unres_us)} thread(s)")
    for kind, who, at, where, body in sorted(new, key=lambda x: x[2]):
        print(f"  {kind} {who} {at} {where}\n      " + body[:400].replace("\n", " "))
