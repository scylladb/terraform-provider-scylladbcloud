#!/usr/bin/env bash
# Local stack for developing the provider: Siren on minicloud (the siren
# repo's make all-local-preflight), siren-frontend in public API mode in
# front of it, and a verified dev user whose credentials dev mode reads.
#
# Usage: scripts/local/stack.sh up|down|status
#
# Environment:
#   SIREN_DIR           siren repo, default ../siren (needs scripts/local/all-local.sh)
#   SIREN_FRONTEND_DIR  siren-frontend repo, default ../siren-frontend
#   FRONTEND_PORT       public API port, default 3011
#   DEV_EMAIL           dev user, default tf-dev@local.test
#   XDG_CONFIG_HOME, XDG_CACHE_HOME  where credentials and state live

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SIREN_DIR="$(cd "${SIREN_DIR:-${ROOT}/../siren}" 2>/dev/null && pwd || echo "${SIREN_DIR:-${ROOT}/../siren}")"
SIREN_FRONTEND_DIR="$(cd "${SIREN_FRONTEND_DIR:-${ROOT}/../siren-frontend}" 2>/dev/null && pwd || echo "${SIREN_FRONTEND_DIR:-${ROOT}/../siren-frontend}")"
FRONTEND_PORT="${FRONTEND_PORT:-3011}"
DEV_EMAIL="${DEV_EMAIL:-tf-dev@local.test}"
DEV_DOMAIN="${DEV_EMAIL#*@}"

CONFIG_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/terraform-provider-scylladbcloud"
STATE_DIR="${XDG_CACHE_HOME:-${HOME}/.cache}/terraform-provider-scylladbcloud/stack"
CREDENTIALS="${CONFIG_DIR}/dev-credentials.json"
FRONTEND_PID="${STATE_DIR}/frontend.pid"
FRONTEND_LOG="${STATE_DIR}/frontend.log"

SIREN_API="http://127.0.0.1:8080/api/v1"
API_CONTAINER="siren-scylla-cloud-api-1"
DB_CONTAINER="dbaas_db"

say() { echo "==> $*"; }
die() {
	echo "error: $*" >&2
	exit 1
}

need() {
	local c
	for c in "$@"; do
		command -v "${c}" >/dev/null || die "${c} is not installed"
	done
}

sql() {
	docker exec "${DB_CONTAINER}" mysql -usiren -p101202 scylla_dbaas -N -e "$1" 2>/dev/null
}

admin() {
	docker exec "${API_CONTAINER}" scylla-cloud "$@" --config /scylla-dbaas.yml --skip-integrity-check >/dev/null 2>&1
}

healthy() { curl -sf --max-time 3 "$1" >/dev/null 2>&1; }

# The public API answers any request, with an error without a token.
frontend_answers() {
	[[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "http://127.0.0.1:${FRONTEND_PORT}/account/default")" != 000 ]]
}

json_get() { python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get(sys.argv[1], (d.get("data") or {}).get(sys.argv[1], "")))' "$1"; }

# allow_dev_domain lets the dev user register: the generated minicloud
# configs are local and git-ignored, so the change stays on this machine.
allow_dev_domain() {
	local f changed=0
	for f in "${SIREN_DIR}/scylla-dbaas.yml.minicloud" "${SIREN_DIR}/scylla-dbaas.yml.minicloud-docker"; do
		[[ -f "${f}" ]] || continue
		grep -q -- "- ${DEV_DOMAIN}\$" "${f}" && continue
		DEV_DOMAIN="${DEV_DOMAIN}" perl -0pi -e \
			's/(    user_config:\n      password_hash_salt: [^\n]*\n)/$1      allowed_registration_domains:\n        - scylladb.com\n        - $ENV{DEV_DOMAIN}\n/' "${f}"
		grep -q -- "- ${DEV_DOMAIN}\$" "${f}" || die "could not add ${DEV_DOMAIN} to ${f}"
		changed=1
	done
	echo "${changed}"
}

siren_up() {
	[[ -x "${SIREN_DIR}/scripts/local/all-local.sh" ]] ||
		die "${SIREN_DIR} has no scripts/local/all-local.sh; use a siren branch with the all-local tooling"
	grep -q "ProgrammaticAccessKeys = !body.NoProgrammaticAccessKeys" "${SIREN_DIR}/scripts/local/byokfake/server.go" ||
		echo "warning: siren's fake BYOK lacks the ProgrammaticAccessKeys fix; clusters with encryption at rest will fail" >&2

	say "siren on minicloud (make all-local-preflight, keeps a running minicloud)"
	MINICLOUD_RESTART=0 make -C "${SIREN_DIR}" all-local-preflight >"${STATE_DIR}/preflight.log" 2>&1 ||
		die "siren preflight failed, see ${STATE_DIR}/preflight.log"

	if [[ "$(allow_dev_domain)" == 1 ]]; then
		say "allowed ${DEV_DOMAIN} sign-ups; restarting siren"
		MINICLOUD_RESTART=0 RESTART_SIREN=1 make -C "${SIREN_DIR}" all-local-preflight >>"${STATE_DIR}/preflight.log" 2>&1 ||
			die "siren restart failed, see ${STATE_DIR}/preflight.log"
	fi
}

frontend_up() {
	if frontend_answers; then
		say "siren-frontend already answers on :${FRONTEND_PORT}"
		return
	fi
	[[ -d "${SIREN_FRONTEND_DIR}/server" ]] || die "no siren-frontend at ${SIREN_FRONTEND_DIR}"
	if [[ ! -d "${SIREN_FRONTEND_DIR}/node_modules" ]]; then
		say "installing siren-frontend dependencies"
		(cd "${SIREN_FRONTEND_DIR}" && pnpm install --frozen-lockfile >"${STATE_DIR}/pnpm.log" 2>&1) ||
			die "pnpm install failed, see ${STATE_DIR}/pnpm.log"
	fi
	say "building siren-frontend server"
	(cd "${SIREN_FRONTEND_DIR}/server" && rm -rf build && NODE_ENV=development node esbuild.config.js >"${STATE_DIR}/build.log" 2>&1) ||
		die "siren-frontend build failed, see ${STATE_DIR}/build.log"

	say "starting siren-frontend (public API) on :${FRONTEND_PORT}"
	(cd "${SIREN_FRONTEND_DIR}/server" && NODE_ENV=development PUBLIC_API=true \
		API_BACKEND_URL=http://127.0.0.1:8088/api/v0 API_WEBHOOK_URL=http://127.0.0.1:8088 \
		SERVER_PORT="${FRONTEND_PORT}" nohup node build/server.js >"${FRONTEND_LOG}" 2>&1 </dev/null &
		echo $! >"${FRONTEND_PID}")
	local i
	for i in $(seq 1 30); do
		frontend_answers && return
		sleep 1
	done
	die "siren-frontend did not start, see ${FRONTEND_LOG}"
}

credentials() {
	if [[ ! -f "${CREDENTIALS}" ]]; then
		say "writing dev credentials to ${CREDENTIALS}"
		(umask 077 && mkdir -p "${CONFIG_DIR}" &&
			python3 -c 'import json,secrets,sys; print(json.dumps({"email": sys.argv[1], "password": "Tf1!" + secrets.token_urlsafe(18)}))' "${DEV_EMAIL}" >"${CREDENTIALS}")
	fi
	chmod 600 "${CREDENTIALS}"
}

dev_user() {
	local email password user_id account_id account_name status billing body
	email="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["email"])' "${CREDENTIALS}")"
	password="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["password"])' "${CREDENTIALS}")"

	user_id="$(sql "select id from user where email='${email}'")"
	if [[ -z "${user_id}" ]]; then
		say "creating dev user ${email}"
		body="$(python3 -c 'import json,sys; print(json.dumps({"Email": sys.argv[1], "Password": sys.argv[2], "FirstName": "TF", "LastName": "Dev", "Phone": "+15555550100", "Company": "TF Dev", "CountryID": 1}))' "${email}" "${password}")"
		# Siren creates the user, then fails to set its country outside a
		# session; the billing step below covers what that would do.
		curl -s --max-time 30 -X POST "${SIREN_API}/user" -H 'Content-Type: application/json' -d "${body}" >/dev/null || true
		user_id="$(sql "select id from user where email='${email}'")"
		[[ -n "${user_id}" ]] || die "siren did not create ${email}; see docker logs ${API_CONTAINER}"
	fi

	status="$(sql "select status from user where id=${user_id}")"
	if [[ "${status}" != ACTIVE ]]; then
		say "verifying dev user"
		admin user verify --user-id "${user_id}" --email "${email}" || die "user verify failed"
	fi

	account_id="$(sql "select default_account_id from user where id=${user_id}")"
	account_name="$(sql "select name from account where id=${account_id}")"
	billing="$(sql "select status from account_billing_status where account_id=${account_id}")"
	if [[ "${billing}" != VERIFIED ]]; then
		say "verifying billing of account ${account_id}"
		admin account billing verify --account-id "${account_id}" --account-name "${account_name}" || die "billing verify failed"
	fi

	body="$(python3 -c 'import json,sys; print(json.dumps({"Email": sys.argv[1], "Password": sys.argv[2]}))' "${email}" "${password}")"
	[[ -n "$(curl -s --max-time 10 -X POST "${SIREN_API}/user/login" -H 'Content-Type: application/json' -d "${body}" | json_get Token)" ]] ||
		die "dev user ${email} cannot log in; if siren's database was recreated, delete ${CREDENTIALS} and run up again"
	say "dev user ${email} (account ${account_id}) can log in"
}

status() {
	local ok=0 vms
	healthy http://127.0.0.1:5050/minicloud/status && echo "minicloud       up (:5050)" || { echo "minicloud       DOWN"; ok=1; }
	healthy http://127.0.0.1:8080/-/healthcheck && echo "siren api       up (:8080)" || { echo "siren api       DOWN"; ok=1; }
	healthy http://127.0.0.1:8088/-/healthcheck && echo "siren public    up (:8088)" || { echo "siren public    DOWN"; ok=1; }
	if frontend_answers; then
		echo "siren-frontend  up (:${FRONTEND_PORT})"
	else
		echo "siren-frontend  DOWN"
		ok=1
	fi
	[[ -f "${CREDENTIALS}" ]] && echo "credentials     ${CREDENTIALS}" || { echo "credentials     missing"; ok=1; }
	vms="$(curl -s --max-time 3 http://127.0.0.1:5050/minicloud/status | python3 -c 'import json,sys; print(json.load(sys.stdin)["running_vms"])' 2>/dev/null || echo "?")"
	echo "minicloud VMs   ${vms}"
	if ((ok == 0)); then
		echo
		echo "export SCYLLADB_CLOUD_DEV_MODE=1 SCYLLADB_CLOUD_ENDPOINT=http://localhost:${FRONTEND_PORT}"
	fi
	return "${ok}"
}

frontend_down() {
	if [[ -f "${FRONTEND_PID}" ]]; then
		kill "$(cat "${FRONTEND_PID}")" 2>/dev/null || true
		rm -f "${FRONTEND_PID}"
		say "stopped siren-frontend"
	fi
}

mkdir -p "${STATE_DIR}"

case "${1:-}" in
up)
	need docker go node pnpm python3 curl perl make
	siren_up
	frontend_up
	credentials
	dev_user
	echo
	status
	;;
down)
	frontend_down
	say "stopping siren, the fake BYOK services and minicloud (make all-local-down)"
	make -C "${SIREN_DIR}" all-local-down
	;;
status)
	status
	;;
*)
	echo "usage: $0 up|down|status" >&2
	exit 2
	;;
esac
