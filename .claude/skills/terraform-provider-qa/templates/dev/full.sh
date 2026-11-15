#!/usr/bin/env bash
# Full regression for one plan row: stack check, testacc-local, then
# release and dev-mode upgrade/lifecycle runs (dev/row.sh).
set -uo pipefail
cd "$(dirname "$0")/.."
echo "### stack"
scripts/local/stack.sh status >/dev/null || { echo "STACK DOWN"; exit 1; }
echo "### testacc-local"
make testacc-local > dev/testacc-local.log 2>&1
rc=$?
grep -E "^(--- |PASS$|FAIL|ok )" dev/testacc-local.log
((rc == 0)) || { echo "FAIL testacc-local"; exit 1; }
dev/row.sh || exit 1
for check in dev/checks/*/check.sh; do
	[[ -x "$check" ]] || continue
	echo "### check $(basename "$(dirname "$check")")"
	"$check" || { echo "FAIL check $check"; exit 1; }
done
echo "### ALL PASS"
