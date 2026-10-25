#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/../../scripts/lib/environment.sh"
topology
cleanup() {
  if k get namespace "$NS" >/dev/null 2>&1; then
    [[ "$(k get namespace "$NS" -o jsonpath='{.metadata.labels.ai\.dhiraj\.io/proof}')" == day2 ]] || fail 'Foreign proof namespace.'
    k delete namespace "$NS" --wait=true --timeout=180s
  fi
  bash "$ROOT/hack/manual-gpu-capacity.sh" remove
}
if [[ "${1:-}" == cleanup ]]; then cleanup; exit; fi
! k get namespace "$NS" >/dev/null 2>&1 || fail 'Proof namespace already exists; inspect and clean it first.'
finish() {
  local result=$?
  trap - EXIT
  k get pods -n "$NS" -o wide > "$EVIDENCE/proof-pods.txt" 2>&1 || true
  k get events -n "$NS" -o json > "$EVIDENCE/proof-events.json" 2>&1 || true
  cleanup || result=1
  exit "$result"
}
k create namespace "$NS"
k label namespace "$NS" ai.dhiraj.io/proof=day2
trap finish EXIT
bash "$ROOT/hack/manual-gpu-capacity.sh" apply
pod() {
  jq -n --arg name "$1" --arg node "$2" --arg ns "$NS" --arg image "${PROOF_IMAGE:-registry.k8s.io/pause:3.10}" --arg r "$RESOURCE" --arg mode "$3" '{apiVersion:"v1",kind:"Pod",metadata:{name:$name,namespace:$ns},spec:{restartPolicy:"Never",nodeSelector:{"kubernetes.io/hostname":$node},containers:[{name:"pause",image:$image,resources:(if $mode=="gpu" then {limits:{($r):"1"},requests:{($r):"1"}} else {} end)}],tolerations:(if $mode=="gpu" then [{key:"ai.dhiraj.io/gpu",operator:"Equal",value:"true",effect:"NoSchedule"}] else [] end)}}' | k create -f -
}
pod cpu "$CPU_NODE" cpu
pod isolated "$GPU_NODE" cpu
for i in 1 2 3; do pod "gpu-$i" "$GPU_NODE" gpu; done
k wait -n "$NS" pod/cpu --for=condition=Ready --timeout=180s
exhausted() {
  k get pods -n "$NS" -o json | jq -e --arg node "$GPU_NODE" '[.items[]|select(.metadata.name|startswith("gpu-"))] as $p | ([$p[]|select(.status.phase=="Running" and .spec.nodeName==$node)]|length)==2 and ([$p[]|select(any(.status.conditions[]?; .type=="PodScheduled" and .status=="False" and (.message|contains("Insufficient ai.dhiraj.io/gpu"))))]|length)==1' >/dev/null
}
wait_for exhausted
k get pod isolated -n "$NS" -o json | jq -e '.spec.nodeName==null and any(.status.conditions[]?; .type=="PodScheduled" and .reason=="Unschedulable")' >/dev/null
pending="$(k get pods -n "$NS" -o json | jq -r '.items[]|select((.metadata.name|startswith("gpu-")) and .status.phase=="Pending")|.metadata.name')"
running="$(k get pods -n "$NS" -o json | jq -r '[.items[]|select((.metadata.name|startswith("gpu-")) and .status.phase=="Running")][0].metadata.name')"
k get pods -n "$NS" -o json > "$EVIDENCE/exhaustion.json"
k get events -n "$NS" -o json > "$EVIDENCE/exhaustion-events.json"
k delete pod "$running" -n "$NS" --wait=true
k wait -n "$NS" "pod/$pending" --for=condition=Ready --timeout=180s
[[ "$(k get pod "$pending" -n "$NS" -o jsonpath='{.spec.nodeName}')" == "$GPU_NODE" ]] || fail 'Unexpected GPU placement.'
[[ "$(k get pod cpu -n "$NS" -o jsonpath='{.spec.nodeName}')" == "$CPU_NODE" ]] || fail 'Unexpected CPU placement.'
echo 'PASS: placement, isolation, exhaustion and release. Cleaning proof resources.'
