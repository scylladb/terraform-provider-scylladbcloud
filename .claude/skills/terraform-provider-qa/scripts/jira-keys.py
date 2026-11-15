#!/usr/bin/env python3
"""Usage: jira-keys.py <pr-status.json> [<plan-status.json>] -> one Jira key per line.
The provider dashboard keeps keys inside text ("Closing CLOUD-5130, CLOUD-5128") or
URLs (".../browse/CLOUD-5128"), and plan rows also name them in `step`."""
import json, re, sys

keys = set()
for p in sys.argv[1:]:
    d = json.load(open(p))
    rows = d["rows"] if isinstance(d, dict) else d
    for r in rows:
        for field in ("jira", "step"):
            keys |= set(re.findall(r"\b[A-Z][A-Z0-9]+-\d+\b", str(r.get(field) or "")))
print("\n".join(sorted(keys, key=lambda k: (k.split("-")[0], int(k.split("-")[1])))))
