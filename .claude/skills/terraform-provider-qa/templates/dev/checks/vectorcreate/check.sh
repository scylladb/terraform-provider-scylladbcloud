#!/usr/bin/env bash
# Live check of scylladbcloud_cluster's create-time vector_search block:
# create with it, verify the install request and nodes, reject a later
# change, allow removing the block, destroy. Local build only.
set -uo pipefail
cd "$(dirname "$0")"
export SCYLLADB_CLOUD_ENDPOINT="${SCYLLADB_CLOUD_ENDPOINT:-http://localhost:3011}" SCYLLADB_CLOUD_DEV_MODE=1
export TF_CLI_CONFIG_FILE="$PWD/../../local.tfrc" TF_IN_AUTOMATION=1 TF_CLI_ARGS="-no-color"
unset SCYLLADB_CLOUD_TOKEN
LOG="../../logs/$(date +%Y%m%d-%H%M%S)-vectorcreate"
mkdir -p "$LOG"
fail() { echo "FAIL  vectorcreate $1 (see $LOG)"; terraform destroy -input=false -auto-approve >"$LOG/cleanup-destroy.log" 2>&1; exit 1; }
pass() { echo "PASS  vectorcreate-$1"; }

rm -f terraform.tfstate*
terraform init -input=false >"$LOG/init.log" 2>&1 || fail init
terraform apply -input=false -auto-approve >"$LOG/apply.log" 2>&1 || fail apply
pass apply
cluster=$(terraform output -raw cluster_id)

# The dev-mode provider refreshes the cached token when it logs in.
token=$(python3 -c 'import json,os;d=json.load(open(os.path.expanduser("~/.cache/terraform-provider-scylladbcloud/dev-token.json")));print(list(d.values())[0]["token"])')
account=$(curl -s -m20 "$SCYLLADB_CLOUD_ENDPOINT/account/default" -H "Authorization: Bearer $token" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["accountId"])') ||
	fail "could not read the account"
api() { curl -s -m20 "$SCYLLADB_CLOUD_ENDPOINT/account/$account$1" -H "Authorization: Bearer $token"; }

api "/cluster/$cluster/request?type=INSTALL_VECTOR_SEARCH" >"$LOG/install-request.json"
python3 -c 'import json,sys;r=json.load(open(sys.argv[1]))["data"];sys.exit(0 if r and all(x["status"]=="COMPLETED" for x in r) else 1)' "$LOG/install-request.json" ||
	fail "install request not COMPLETED"
pass install-request-completed

dc=$(api "/cluster/$cluster/dc" | tee "$LOG/dcs.json" | python3 -c '
import json,sys
d=json.load(sys.stdin)["data"]
dcs=d if isinstance(d,list) else next(v for v in d.values() if isinstance(v,list))
print(dcs[0]["id"])')
nodes=$(api "/cluster/$cluster/dc/$dc/vector-search" | tee "$LOG/vector-search.json" | python3 -c '
import json,sys
d=json.load(sys.stdin)["data"]
print(sum(len(z.get("nodes") or []) for z in d.get("availabilityZones") or []))')
[[ "$nodes" == 1 ]] || fail "api reports $nodes vector search nodes, want 1"
pass nodes-1

terraform plan -input=false -detailed-exitcode >"$LOG/plan.log" 2>&1 || fail "plan not clean"
pass plan-clean

terraform plan -input=false -var vs_nodes=2 >"$LOG/plan-resize.log" 2>&1 && fail "resize plan was accepted"
grep -q "applied only when the cluster is created" "$LOG/plan-resize.log" || fail "resize plan failed for another reason"
pass resize-rejected

terraform plan -input=false -detailed-exitcode -var vs_nodes=0 >"$LOG/plan-remove.log" 2>&1
rc=$?
[[ $rc == 2 ]] && grep -q "0 to add, 1 to change, 0 to destroy" "$LOG/plan-remove.log" || fail "remove plan (exit $rc) is not an in-place update"
terraform apply -input=false -auto-approve -var vs_nodes=0 >"$LOG/apply-remove.log" 2>&1 || fail apply-remove
terraform plan -input=false -detailed-exitcode -var vs_nodes=0 >"$LOG/plan-removed.log" 2>&1 || fail "plan after removal not clean"
nodes=$(api "/cluster/$cluster/dc/$dc/vector-search" | python3 -c '
import json,sys
d=json.load(sys.stdin)["data"]
print(sum(len(z.get("nodes") or []) for z in d.get("availabilityZones") or []))')
[[ "$nodes" == 1 ]] || fail "removing the block changed the nodes to $nodes"
pass block-removed-nodes-kept

terraform destroy -input=false -auto-approve -var vs_nodes=0 >"$LOG/destroy.log" 2>&1 || fail destroy
pass destroy
