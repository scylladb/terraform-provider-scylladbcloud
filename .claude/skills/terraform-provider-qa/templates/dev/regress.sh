#!/usr/bin/env bash
# Upgrade + lifecycle regression against the local stack.
# Usage: dev/regress.sh [all|release|local|destroy]
# DEV_MODE=1 runs the local phases with the dev build's auto-login (no token).
set -uo pipefail
cd "$(dirname "$0")"
export SCYLLADB_CLOUD_ENDPOINT="${SCYLLADB_CLOUD_ENDPOINT:-http://localhost:3011}"
export SCYLLADB_CLOUD_TOKEN="${SCYLLADB_CLOUD_TOKEN:-$(cat "$HOME/.cache/terraform-provider-scylladbcloud/dev-token")}"
export TF_IN_AUTOMATION=1 TF_CLI_ARGS="-no-color"
LOG="logs/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$LOG"
results=()

step() {
	local name="$1" want="$2" rc
	shift 2
	echo "== $name: $*"
	"$@" >"$LOG/$name.log" 2>&1
	rc=$?
	if [[ " $want " == *" $rc "* ]]; then
		results+=("PASS  $name (exit $rc)")
	else
		results+=("FAIL  $name (exit $rc, want $want) see $LOG/$name.log")
		tail -20 "$LOG/$name.log"
		summary
		exit 1
	fi
}

summary() {
	echo "-- summary ($LOG)"
	printf '%s\n' "${results[@]}"
}

release() {
	# Files a failed earlier run may have left behind; v1.13.0 rejects them.
	rm -f vector.tf datasources.tf
	export TF_CLI_CONFIG_FILE="$PWD/release.tfrc"
	step release-init 0 terraform init -input=false -upgrade
	step release-apply 0 terraform apply -input=false -auto-approve
	step release-plan 0 terraform plan -input=false -detailed-exitcode
}

use_local() {
	export TF_CLI_CONFIG_FILE="$PWD/local.tfrc"
	if [[ "${DEV_MODE:-0}" == 1 ]]; then
		unset SCYLLADB_CLOUD_TOKEN
		export SCYLLADB_CLOUD_DEV_MODE=1
	fi
}

local_build() {
	use_local
	step upgrade-plan 0 terraform plan -input=false -detailed-exitcode
	step upgrade-plan-norefresh 0 terraform plan -input=false -detailed-exitcode -refresh=false
	step upgrade-refresh 0 terraform apply -input=false -refresh-only -auto-approve
	step local-apply 0 terraform apply -input=false -auto-approve
	step local-plan 0 terraform plan -input=false -detailed-exitcode
	step rf-state 0 sh -c 'terraform state show scylladbcloud_cluster.dev | grep -E "replication_factor += 3$"'
	rename
	encryption_mode
	combined
	if [[ "${SCYLLADB_CLOUD_LOCAL_VECTOR:-0}" == 1 ]]; then
		vector_search
	fi
	if [[ -n "${DS_ROW:-}" ]]; then
		datasources
	fi
}

# datasources reads the data sources of plan row DS_ROW against the cluster;
# they live in a file that exists only during this step.
datasources() {
	./datasources-tf.sh "$DS_ROW" >datasources.tf
	step ds-apply 0 terraform apply -input=false -auto-approve
	step ds-plan 0 terraform plan -input=false -detailed-exitcode
	step ds-outputs 0 sh -c 'terraform output -json | python3 ds_check.py'
	rm -f datasources.tf
	step ds-removed 0 terraform apply -input=false -auto-approve
}

# vector_search adds one vector search node to the cluster, checks the plan is
# clean, then removes it. The resource is unknown to the release build, so it
# lives in a file that exists only during this step.
vector_search() {
	cat >vector.tf <<'TF'
resource "scylladbcloud_vector_search" "dev" {
  cluster_id = scylladbcloud_cluster.dev.cluster_id
  datacenter = scylladbcloud_cluster.dev.datacenter
  node_count = 1
  node_type  = "t4g.medium"
}
TF
	step vector-apply 0 terraform apply -input=false -auto-approve
	step vector-plan 0 terraform plan -input=false -detailed-exitcode
	rm -f vector.tf
	step vector-remove 0 terraform apply -input=false -auto-approve
	step vector-removed-plan 0 terraform plan -input=false -detailed-exitcode
}

# combined renames and changes client encryption in one apply, then undoes
# both the same way; each must be a single in-place update.
combined() {
	local name mode
	for pair in tf-dev-combo:encrypted tf-dev:both; do
		name="${pair%%:*}" mode="${pair#*:}"
		printf 'resource "scylladbcloud_cluster" "dev" {\n  client_encryption_mode = "%s"\n}\n' "${mode}" >encmode_override.tf
		step "combined-plan-${name}" 2 terraform plan -input=false -detailed-exitcode -var "cluster_name=${name}" -out combined.tfplan
		if ! grep -q "0 to add, 1 to change, 0 to destroy" "$LOG/combined-plan-${name}.log"; then
			results+=("FAIL  combined-plan-${name} (not an in-place update)")
			rm -f encmode_override.tf
			summary
			exit 1
		fi
		step "combined-apply-${name}" 0 terraform apply -input=false -auto-approve combined.tfplan
		step "combined-check-${name}" 0 terraform plan -input=false -detailed-exitcode -var "cluster_name=${name}"
	done
	rm -f encmode_override.tf combined.tfplan
}

# encryption_mode switches client encryption to encrypted and back to both,
# each an in-place update. The attribute is unknown to the release build, so
# it is set only here, through an override file.
encryption_mode() {
	local mode
	for mode in encrypted both; do
		printf 'resource "scylladbcloud_cluster" "dev" {\n  client_encryption_mode = "%s"\n}\n' "${mode}" >encmode_override.tf
		step "encmode-plan-${mode}" 2 terraform plan -input=false -detailed-exitcode -out encmode.tfplan
		if ! grep -q "0 to add, 1 to change, 0 to destroy" "$LOG/encmode-plan-${mode}.log"; then
			results+=("FAIL  encmode-plan-${mode} (not an in-place update)")
			tail -20 "$LOG/encmode-plan-${mode}.log"
			rm -f encmode_override.tf
			summary
			exit 1
		fi
		step "encmode-ca-planned-${mode}" 0 sh -c 'terraform show -json encmode.tfplan | python3 -c "import json,sys;c=[x for x in json.load(sys.stdin)[\"resource_changes\"] if x[\"address\"]==\"scylladbcloud_cluster.dev\"][0][\"change\"];sys.exit(0 if c[\"after_unknown\"].get(\"ca_certificate\") is True else 1)"'
		step "encmode-apply-${mode}" 0 terraform apply -input=false -auto-approve encmode.tfplan
		step "encmode-ca-state-${mode}" 0 sh -c 'terraform show -json | python3 -c "import json,sys;r=[x for x in json.load(sys.stdin)[\"values\"][\"root_module\"][\"resources\"] if x[\"address\"]==\"scylladbcloud_cluster.dev\"][0];sys.exit(0 if r[\"values\"][\"ca_certificate\"].startswith(\"-----BEGIN\") else 1)"'
		step "encmode-check-${mode}" 0 terraform plan -input=false -detailed-exitcode
	done
	rm -f encmode_override.tf encmode.tfplan
	step encmode-unset 0 terraform plan -input=false -detailed-exitcode
}

# rename checks that a new cluster name is applied in place, then renames back.
rename() {
	local name
	for name in tf-dev-renamed tf-dev; do
		step "rename-plan-${name}" 2 terraform plan -input=false -detailed-exitcode -var "cluster_name=${name}" -out "rename.tfplan"
		if ! grep -q "0 to add, 1 to change, 0 to destroy" "$LOG/rename-plan-${name}.log"; then
			results+=("FAIL  rename-plan-${name} (not an in-place update)")
			tail -20 "$LOG/rename-plan-${name}.log"
			summary
			exit 1
		fi
		step "rename-apply-${name}" 0 terraform apply -input=false -auto-approve rename.tfplan
		step "rename-check-${name}" 0 terraform plan -input=false -detailed-exitcode -var "cluster_name=${name}"
	done
	rm -f rename.tfplan
}

destroy() {
	use_local
	step destroy 0 terraform destroy -input=false -auto-approve
}

case "${1:-all}" in
all) release && local_build && destroy ;;
release) release ;;
local) local_build ;;
destroy) destroy ;;
*) echo "usage: $0 [all|release|local|destroy]" >&2; exit 2 ;;
esac
summary
