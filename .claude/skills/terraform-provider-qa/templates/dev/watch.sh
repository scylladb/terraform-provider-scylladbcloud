#!/usr/bin/env bash
# Prints one status line every 30s for a dev/full.sh run; exits when it ends.
F="$1"
while true; do
  phase=$(grep '^### ' "$F" | tail -1 | sed 's/^### //')
  step=$(grep '^== ' "$F" | tail -1 | sed 's/^== //; s/:.*//')
  vms=$(curl -s -m3 http://127.0.0.1:5050/minicloud/status | python3 -c 'import json,sys;print(json.load(sys.stdin)["running_vms"])' 2>/dev/null)
  req=$(docker exec dbaas_db mysql -usiren -p101202 scylla_dbaas -N -e "select concat(id,' ',request_type,' ',status,' ',progress_percent,'% ',progress_description) from cluster_request where request_type not like '%CHECK%' order by id desc limit 1" 2>/dev/null)
  echo "$(date +%H:%M:%S) phase=[$phase] step=[$step] vms=${vms:-?} req=[$req]"
  if grep -qE '^### ALL PASS$|^FAIL |^UNIT FAIL$|^STACK DOWN$|^--- FAIL' "$F"; then echo "DONE: $(grep -E '^(FAIL|###|--- FAIL)' "$F" | tr '\n' ';')"; break; fi
  sleep 30
done
