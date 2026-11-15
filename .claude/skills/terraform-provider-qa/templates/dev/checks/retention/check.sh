#!/usr/bin/env bash
# Live check of scylladbcloud_cluster_backup_retention against the newest
# deleted cluster that still has a retention. Local build only.
set -uo pipefail
cd "$(dirname "$0")"
export SCYLLADB_CLOUD_ENDPOINT="${SCYLLADB_CLOUD_ENDPOINT:-http://localhost:3011}" SCYLLADB_CLOUD_DEV_MODE=1
export TF_CLI_CONFIG_FILE="$PWD/../../local.tfrc" TF_IN_AUTOMATION=1 TF_CLI_ARGS="-no-color"
unset SCYLLADB_CLOUD_TOKEN
LOG="../../logs/$(date +%Y%m%d-%H%M%S)-retention"
mkdir -p "$LOG"
fail() { echo "FAIL  retention $1 (see $LOG)"; exit 1; }

token=$(python3 -c 'import json,os;d=json.load(open(os.path.expanduser("~/.cache/terraform-provider-scylladbcloud/dev-token.json")));print(list(d.values())[0]["token"])')
account=$(curl -s -m10 "$SCYLLADB_CLOUD_ENDPOINT/account/default" -H "Authorization: Bearer $token" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["accountId"])') ||
	fail "could not read the account"
cluster=""
for id in $(docker exec dbaas_db mysql -usiren -p101202 scylla_dbaas -N -e "select id from cluster where account_id=$account and status='DELETED' order by id desc limit 5" 2>/dev/null); do
	if curl -s -m10 "$SCYLLADB_CLOUD_ENDPOINT/account/$account/cluster/$id/backup-retention" -H "Authorization: Bearer $token" | grep -q retentionDays; then
		cluster=$id
		break
	fi
done
[[ -n "$cluster" ]] || fail "no deleted cluster with a backup retention"
echo "== retention: cluster $cluster"
rm -f terraform.tfstate*
terraform init -input=false >"$LOG/init.log" 2>&1 || fail init
for days in 2 3; do
	terraform apply -input=false -auto-approve -var "cluster_id=$cluster" -var "days=$days" >"$LOG/apply-$days.log" 2>&1 || fail "apply-$days"
	terraform plan -input=false -detailed-exitcode -var "cluster_id=$cluster" -var "days=$days" >"$LOG/plan-$days.log" 2>&1 || fail "plan-$days"
	got=$(curl -s -m10 "$SCYLLADB_CLOUD_ENDPOINT/account/$account/cluster/$cluster/backup-retention" -H "Authorization: Bearer $token" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["retentionDays"])')
	[[ "$got" == "$days" ]] || fail "api reports $got days, want $days"
	echo "PASS  retention-$days (exit 0)"
done
terraform destroy -input=false -auto-approve -var "cluster_id=$cluster" -var "days=3" >"$LOG/destroy.log" 2>&1 || fail destroy
echo "PASS  retention-destroy (exit 0)"
