#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
topology
[[ ! -d "$ROOT/work/day-09/active" ]] || fail 'A demo/E2E run is active.'
k get aiworkloads -A -o json | jq -e '.items|length==0' >/dev/null || fail 'Remove only your known AIWorkloads before lifecycle testing.'
out="$ROOT/work/day-10/lifecycle/$(date -u +%Y%m%dT%H%M%SZ)-$$"
mkdir -p "$out"
exec > >(tee "$out/run.log") 2>&1
ns="$(printf '%s' '{"apiVersion":"v1","kind":"Namespace","metadata":{"generateName":"gpu-day10-persist-"}}' | k create -f - -o jsonpath='{.metadata.name}')"
cleanup() {
  local result=$?
  trap - EXIT
  k -n "$ns" get aiworkloads,configmaps -o json > "$out/final.json" 2>&1 || true
  k delete namespace "$ns" --wait=true --timeout=120s || result=1
  printf '%s\n' "$result" > "$out/exit-code"
  echo "Lifecycle evidence: $out (exit $result)"
  exit "$result"
}
trap cleanup EXIT
k -n "$ns" create configmap retained-data --from-literal=marker="$ns"
jq -n --arg ns "$ns" '{apiVersion:"ai.dhiraj.io/v1alpha1",kind:"AIWorkload",metadata:{name:"retained",namespace:$ns},spec:{image:"gpu-node-selector/mock-inference:dev",replicas:0,gpu:{product:"A100",count:1}}}' | k create -f -
k -n "$ns" wait --for=condition=Ready aiworkload/retained --timeout=180s
identity() {
  k -n "$ns" get aiworkload retained -o json | jq -S '{uid:.metadata.uid,spec}'
  k -n "$ns" get configmap retained-data -o json | jq -S '{uid:.metadata.uid,data}'
  k -n ai-system get deployment aiworkload-operator -o json | jq -S '{uid:.metadata.uid}'
}
identity > "$out/before.json"
bash "$ROOT/scripts/cluster-down.sh"
bash "$ROOT/scripts/cluster-down.sh"
colima list --json | tee "$out/stopped-vms.json" | jq -se --arg p "$PROFILE" 'any(.[]; .name==$p and .status=="Stopped")' >/dev/null
[[ -f "$HOME/.minikube/profiles/$PROFILE/config.json" && -d "$HOME/.colima/_lima/_disks/colima-$PROFILE" ]] || fail 'Stop removed profile/data disk.'
bash "$ROOT/scripts/cluster-up.sh"
if [[ "$(k -n ai-system get deployment aiworkload-operator -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}')" == Helm ]]; then
  make -C "$ROOT" helm-install
else
  make -C "$ROOT" deploy
fi
k -n "$ns" wait --for=condition=Ready aiworkload/retained --timeout=180s
identity > "$out/after.json"
cmp "$out/before.json" "$out/after.json" || fail 'Object identity or data changed across stop/start.'
k -n "$ns" get aiworkload retained -o json | jq -e '.status.observedGeneration==.metadata.generation and .status.phase=="Running" and any(.status.conditions[]; .type=="Ready" and .reason=="ScaledToZero")' >/dev/null
echo 'PASS: verified shutdown, repeated stop, preserved object UIDs/spec/data and restart/deploy.'
