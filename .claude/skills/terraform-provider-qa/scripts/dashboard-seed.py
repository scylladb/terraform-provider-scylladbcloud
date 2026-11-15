#!/usr/bin/env python3
"""Usage: dashboard-seed.py <out-dir> --title T [--prs /_blob/ID] [--plan /_blob/ID] [--live /_blob/ID] [--page]
Writes one JSON document per store write into <out-dir> and prints the ArtifactData
`batch` writes array that sends them (each entry uses file_path). Pass a dataset's blob
url to write that dataset, --page to write dash/meta and files/index.html.
Add if_version to an entry yourself when that document already exists."""
import datetime, json, os, subprocess, sys

here = os.path.dirname(os.path.abspath(__file__))
tpl = os.path.join(here, "..", "templates", "dashboard")
args = sys.argv[1:]
if not args:
    sys.exit(__doc__)
out, title, blobs, page = args.pop(0), "", {}, False
while args:
    a = args.pop(0)
    if a == "--title":
        title = args.pop(0)
    elif a in ("--prs", "--plan", "--live"):
        blobs[a[2:]] = args.pop(0)
    elif a == "--page":
        page = True
    else:
        sys.exit(f"unknown argument {a}")
os.makedirs(out, exist_ok=True)
now = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
stamp = {"via": "claude", "at": now}
repo = subprocess.run([sys.executable, os.path.join(here, "config.py"), "get", "repo"],
                      capture_output=True, text=True).stdout.strip()
writes = []


def put(collection, doc_id, data):
    path = os.path.abspath(os.path.join(out, f"{collection.replace('/', '_')}-{doc_id}.json"))
    json.dump(data, open(path, "w"), indent=1, ensure_ascii=False)
    writes.append({"op": "set", "collection": collection, "doc_id": doc_id, "file_path": path})


if page:
    if not title or not repo:
        sys.exit("--page needs --title and a repo in config")
    put("dash", "meta", {"title": title})
    html = open(os.path.join(tpl, "files", "index.html")).read()
    put("files", "index.html", {"text": html.replace("@TITLE@", title).replace("@REPO@", repo), "edit": stamp})
specs = json.load(open(os.path.join(tpl, "datasets.json")))
for ds, url in blobs.items():
    s = specs[ds]
    put("datasets", ds, {"title": s["title"], "description": s["description"],
                         "source": {"kind": "file", "format": "json", "name": s["name"], "url": url},
                         "updated": {"at": now, "by": "Claude"}, "edit": stamp})
print(json.dumps(writes, indent=1))
