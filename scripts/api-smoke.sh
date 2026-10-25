#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster
mkdir -p "$ROOT/work/day-03"
ns="$(jq -n '{apiVersion:"v1",kind:"Namespace",metadata:{generateName:"ai-day3-"}}' | k create -f - -o jsonpath='{.metadata.name}')"
cleanup() { k delete namespace "$ns" --wait=true --timeout=120s; }
trap cleanup EXIT
jq -n --arg ns "$ns" '{apiVersion:"ai.dhiraj.io/v1alpha1",kind:"AIWorkload",metadata:{name:"minimal",namespace:$ns},spec:{image:"example/mock:dev",gpu:{product:"A100",count:1}}}' | k create -f -
k get aiworkload minimal -n "$ns" -o json | tee "$ROOT/work/day-03/live-defaults.json" | jq -e '.spec.replicas==1 and .spec.port==8080'
if k patch aiworkload minimal -n "$ns" --type=merge -p '{"spec":{"gpu":{"count":0}}}' > "$ROOT/work/day-03/live-invalid.txt" 2>&1; then
  fail 'Invalid GPU count was accepted.'
fi
grep -q 'Invalid value' "$ROOT/work/day-03/live-invalid.txt" || fail 'Rejection was not a validation error.'
k explain aiworkload.spec > "$ROOT/work/day-03/explain.txt"
echo 'PASS: live CRD defaults and validation; removing the temporary namespace.'
