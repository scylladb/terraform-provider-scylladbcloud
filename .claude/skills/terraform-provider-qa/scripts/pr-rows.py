#!/usr/bin/env python3
"""Usage: pr-rows.py <pr-status.json> [--repo owner/repo] [--add N ...]
Rewrites, in place, the fields of each PR row that come straight from GitHub:
title, state, base, commit, ci, checks, failed, additions, deletions, files, size, checked.
Every other field (rows, reviews, top, jira, jira_state, anything another session added)
is kept. --add N appends a row for a PR the dashboard does not have yet.
Prints one line per row that changed."""
import datetime, json, os, subprocess, sys

REPO = subprocess.run([sys.executable, os.path.join(os.path.dirname(os.path.abspath(__file__)), "config.py"), "get", "repo"],
                      capture_output=True, text=True).stdout.strip()
FIELDS = "number,title,url,state,isDraft,baseRefName,headRefOid,additions,deletions,changedFiles,statusCheckRollup"
BAD = ("FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE")
SKIP = ("SKIPPED", "NEUTRAL")

args = sys.argv[1:]
path, add = args.pop(0), []
while args:
    a = args.pop(0)
    if a == "--repo":
        REPO = args.pop(0)
    elif a == "--add":
        while args and args[0].isdigit():
            add.append(int(args.pop(0)))
    else:
        sys.exit(f"unknown argument {a}")
if not REPO:
    sys.exit("no repo: pass --repo owner/repo or run config.py init")


def gh(n):
    out = subprocess.run(["gh", "pr", "view", str(n), "--repo", REPO, "--json", FIELDS],
                         capture_output=True, text=True)
    if out.returncode:
        print(f"!! #{n}: {out.stderr.strip()}", file=sys.stderr)
        return None
    return json.loads(out.stdout)


def fresh(pr):
    passed = failed = running = skipped = 0
    names = []
    for c in pr.get("statusCheckRollup") or []:
        res = c.get("conclusion") or c.get("state") or ""
        if c.get("__typename") == "CheckRun" and c.get("status") != "COMPLETED":
            running += 1
        elif res in ("PENDING", "EXPECTED"):
            running += 1
        elif res in BAD:
            failed += 1
            names.append(c.get("name") or c.get("context"))
        elif res in SKIP:
            skipped += 1
        else:
            passed += 1
    total = passed + failed + running + skipped
    ci = "no checks" if not total else "failing" if failed else "running" if running else "passing"
    state = {"MERGED": "merged", "CLOSED": "closed"}.get(pr["state"], "draft" if pr["isDraft"] else "ready")
    return {
        "title": pr["title"], "url": pr["url"], "state": state, "base": pr["baseRefName"],
        "commit": pr["headRefOid"][:7], "ci": ci,
        "checks": f"{passed} passed, {failed} failed, {running} running, {skipped} skipped",
        "failed": ", ".join(sorted(set(names))),
        "additions": pr["additions"], "deletions": pr["deletions"], "files": pr["changedFiles"],
        "size": f"+{pr['additions']} −{pr['deletions']} · {pr['changedFiles']} files",
    }


doc = json.load(open(path))
rows = doc["rows"] if isinstance(doc, dict) else doc
have = {int(r["pr"]) for r in rows}
for n in add:
    if n not in have:
        rows.append({"pr": n, "rows": "", "reviews": "", "top": "", "jira": "", "jira_state": ""})
now = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
for r in rows:
    pr = gh(r["pr"])
    if pr is None:
        continue
    new = fresh(pr)
    diff = [f"{k}: {r.get(k)!r} -> {v!r}" for k, v in new.items() if r.get(k) != v]
    r.update(new)
    r["checked"] = now
    print(f"#{r['pr']}: " + ("; ".join(diff) if diff else "unchanged"))
rows.sort(key=lambda r: int(r["pr"]))
json.dump(doc, open(path, "w"), indent=1, ensure_ascii=False)
open(path, "a").write("\n")
