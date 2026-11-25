#!/usr/bin/env bash
# Smoke-tests the release files in DIR before they are published: the
# checksums, the signature, and the provider binary of this platform with
# Terraform. Calls no ScyllaDB Cloud API and needs no token. The release
# workflow runs it in its build job. See RELEASING.md.
#
# Usage: scripts/release-smoke.sh DIR VERSION
#   DIR      the release files: the zips, SHA256SUMS, its .sig and the manifest
#   VERSION  the version without "v", e.g. 1.13.1
#
# Needs terraform, gpg with the public signing key, jq and unzip.
set -euo pipefail

if [ $# -ne 2 ]; then
	echo "usage: $0 DIR VERSION" >&2
	exit 2
fi
dir="$(cd "$1" && pwd)"
version="$2"
name=terraform-provider-scylladbcloud
address=registry.terraform.io/scylladb/scylladbcloud

fail() {
	if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::error::$1"; else echo "error: $1" >&2; fi
	exit 1
}

cd "$(git rev-parse --show-toplevel)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

sums="${name}_${version}_SHA256SUMS"
[ -f "$dir/$sums" ] || fail "No ${sums} in ${dir}."
(cd "$dir" && sha256sum -c --quiet "$sums") || fail "A release file does not match SHA256SUMS."
gpg --verify "$dir/${sums}.sig" "$dir/$sums" 2>/dev/null || fail "The signature of SHA256SUMS is not valid."
echo "Checksums and signature: ok"

# Terraform installs the provider from the release zip itself, as from the
# Registry, and every other provider from the Registry.
platform="$(terraform version -json | jq -r .platform)"
zip="${name}_${version}_${platform}.zip"
[ -f "$dir/$zip" ] || fail "No ${zip} in the release files."
mkdir -p "$work/mirror/${address}"
cp "$dir/$zip" "$work/mirror/${address}/"
cat >"$work/terraformrc" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "$work/mirror"
    include = ["$address"]
  }
  direct {
    exclude = ["$address"]
  }
}
EOF
export TF_CLI_CONFIG_FILE="$work/terraformrc" CHECKPOINT_DISABLE=1 TF_IN_AUTOMATION=1
unset TF_PLUGIN_CACHE_DIR

versions_tf() {
	cat <<EOF
terraform {
  required_providers {
    scylladbcloud = {
      source  = "scylladb/scylladbcloud"
      version = "= ${version}"
    }
  }
}
EOF
}

# The provider starts and serves every resource and data source that has a
# docs page.
mkdir "$work/schema"
versions_tf >"$work/schema/versions.tf"
terraform -chdir="$work/schema" init -input=false >/dev/null || fail "terraform init failed with ${zip}."
terraform -chdir="$work/schema" providers schema -json >"$work/schema.json"
for kind in resource data_source; do
	docs=docs/resources
	if [ "$kind" = data_source ]; then docs=docs/data-sources; fi
	served="$(jq -r --arg a "$address" ".provider_schemas[\$a].${kind}_schemas | keys[]" "$work/schema.json")"
	for page in "$docs"/*.md; do
		want="scylladbcloud_$(basename "$page" .md)"
		grep -qx "$want" <<<"$served" || fail "The provider does not serve the ${kind/_/ } ${want}."
	done
	echo "Serves the ${kind/_/ }s: $(tr '\n' ' ' <<<"$served")"
done

# Each example is valid for this build. The docs show every file on its
# own, so each is validated on its own. Validation calls no API.
n=0
for example in $(find examples -name '*.tf' | sort); do
	n=$((n + 1))
	copy="$work/examples/$n"
	mkdir -p "$copy"
	cp "$example" "$copy/"
	if ! grep -q required_providers "$example"; then versions_tf >"$copy/versions.tf"; fi
	for step in "init -input=false -backend=false" validate; do
		# shellcheck disable=SC2086
		if ! out="$(terraform -chdir="$copy" $step -no-color 2>&1)"; then
			echo "$out"
			fail "${example} is not valid for this build."
		fi
	done
	echo "Valid: ${example}"
done
echo "Smoke test of ${zip}: ok"
