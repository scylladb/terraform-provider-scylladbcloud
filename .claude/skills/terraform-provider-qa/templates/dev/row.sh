#!/usr/bin/env bash
# Full regression for one plan row: release build, then dev build with dev mode.
set -uo pipefail
cd "$(dirname "$0")/.."
echo "### unit"
make test >/dev/null && make test-dev >/dev/null && make check-release || { echo "UNIT FAIL"; exit 1; }
echo "### release build"
go build -o "$(go env GOPATH)/bin/terraform-provider-scylladbcloud" . || exit 1
dev/regress.sh all || exit 1
echo "### dev build, DEV_MODE=1"
make build-dev >/dev/null || exit 1
DEV_MODE=1 dev/regress.sh all || exit 1
echo "### ROW PASS"
