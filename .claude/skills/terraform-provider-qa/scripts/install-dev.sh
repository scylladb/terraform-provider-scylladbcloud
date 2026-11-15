#!/usr/bin/env bash
# Usage: install-dev.sh
# Installs the regression workspace (templates/dev) into <repo>/dev. Never overwrites:
# a file that already exists is left as it is and reported "kept".
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
src="$here/../templates/dev"
root=$(git -C "$here" rev-parse --show-toplevel)
dst="$root/dev"

git -C "$root" check-ignore -q dev/ || { echo "FAIL: dev/ is not git-ignored in $root; add /dev/ to .gitignore first"; exit 1; }

dev_email=$(python3 -c 'import json,os;print(json.load(open(os.path.expanduser("~/.config/terraform-provider-scylladbcloud/dev-credentials.json"))).get("email",""))' 2>/dev/null || true)
dev_email=${dev_email:-tf-dev@local.test}
gobin="$(go env GOPATH)/bin"

cd "$src"
find . -type f | sort | while read -r f; do
	out="$dst/${f#./}"; out=${out%.tmpl}
	if [[ -e "$out" ]]; then echo "kept      ${out#"$root"/}"; continue; fi
	mkdir -p "$(dirname "$out")"
	sed -e "s|@GOBIN@|$gobin|g" -e "s|@DEV_EMAIL@|$dev_email|g" "$f" > "$out"
	[[ -x "$f" ]] && chmod +x "$out"
	echo "installed ${out#"$root"/}"
done
mkdir -p "$dst/logs"
