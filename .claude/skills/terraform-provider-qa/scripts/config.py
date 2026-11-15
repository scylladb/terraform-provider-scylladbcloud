#!/usr/bin/env python3
"""Usage: config.py path | show | init | get <key> | set <key> <value>
Per-user config and state of the terraform-provider-qa skill, kept in
<repo>/.claude/terraform-provider-qa/config.json (git-excluded).
`init` fills every key it can detect and keeps the ones already set."""
import json, os, subprocess, sys

ROOT = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True,
                      cwd=os.path.dirname(os.path.abspath(__file__))).stdout.strip()
PATH = os.path.join(ROOT, ".claude", "terraform-provider-qa", "config.json")
KEYS = ("repo", "github_user", "dashboard_url", "combined_branch", "worktree_root", "siren_dir",
        "minicloud_dir", "siren_frontend_dir", "aws_profile", "jira_cloud_id", "last_scan")


def load():
    try:
        return json.load(open(PATH))
    except FileNotFoundError:
        return {}


def save(cfg):
    os.makedirs(os.path.dirname(PATH), exist_ok=True)
    json.dump(cfg, open(PATH, "w"), indent=1)
    open(PATH, "a").write("\n")


def sh(*cmd):
    r = subprocess.run(cmd, capture_output=True, text=True, cwd=ROOT)
    return r.stdout.strip() if r.returncode == 0 else ""


def detect():
    parent, name = os.path.dirname(ROOT), os.path.basename(ROOT)
    return {
        "repo": sh("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"),
        "github_user": sh("gh", "api", "user", "-q", ".login"),
        "combined_branch": sh("git", "branch", "--show-current"),
        "worktree_root": os.path.join(parent, f"{name}-wt"),
        "siren_dir": os.path.join(parent, "siren"),
        "minicloud_dir": os.path.join(parent, "minicloud"),
        "siren_frontend_dir": os.path.join(parent, "siren-frontend"),
        "aws_profile": os.environ.get("AWS_PROFILE", ""),
        "last_scan": "2000-01-01T00:00:00Z",
    }


args = sys.argv[1:]
cmd = args.pop(0) if args else "show"
cfg = load()
if cmd == "path":
    print(PATH)
elif cmd == "show":
    print(json.dumps({k: cfg.get(k, "") for k in KEYS}, indent=1))
elif cmd == "init":
    for k, v in detect().items():
        if not cfg.get(k) and v:
            cfg[k] = v
    save(cfg)
    print(json.dumps({k: cfg.get(k, "") for k in KEYS}, indent=1))
elif cmd == "get" and len(args) == 1:
    if args[0] not in KEYS:
        sys.exit(f"unknown key {args[0]}")
    print(cfg.get(args[0], ""))
elif cmd == "set" and len(args) == 2:
    if args[0] not in KEYS:
        sys.exit(f"unknown key {args[0]}")
    cfg[args[0]] = args[1]
    save(cfg)
else:
    sys.exit(__doc__)
