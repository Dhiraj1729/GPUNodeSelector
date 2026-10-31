#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/lib/environment.sh"
mode="${1:-baseline}"
case "$mode" in baseline|recovery) ;; *) fail 'Usage: e2e.sh baseline|recovery' ;; esac
topology
run="$(date -u +%Y%m%dT%H%M%SZ)-$$-$mode"
out="${E2E_EVIDENCE_ROOT:-$ROOT/work/day-09}/$run"
mkdir -p "$out"
exec > >(tee "$out/run.log") 2>&1
# Only one suite may mutate this project at a time. Never remove another run's lock.
lock="$ROOT/work/day-09/active"
mkdir -p "$(dirname "$lock")"
mkdir "$lock" 2>/dev/null || fail "E2E already active; inspect $lock before removing a stale lock."
printf '%s\n' "pid=$$ run=$run" > "$lock/owner"
ns=''
snapshot() {
  local stage="$1"
  k get nodes -o json > "$out/$stage-nodes.json" || true
  if [[ -n "$ns" ]]; then
    k -n "$ns" get aiworkloads,deployments,services,replicasets,pods -o json > "$out/$stage.json" || true
    k -n "$ns" get events -o json > "$out/$stage-events.json" || true
  fi
  k -n ai-system logs deployment/aiworkload-operator --tail=200 > "$out/$stage-operator.log" 2>&1 || true
  k -n ai-system logs daemonset/fake-gpu-device-plugin --tail=200 > "$out/$stage-plugin.log" 2>&1 || true
}
cleanup() {
  local result=$?
  trap - EXIT
  snapshot final
  if [[ -n "$ns" ]]; then
    k delete namespace "$ns" --wait=true --timeout=120s || result=1
  fi
  rm "$lock/owner"
  rmdir "$lock"
  printf '%s\n' "$result" > "$out/exit-code"
  echo "Evidence: $out (exit $result)"
  exit "$result"
}
trap cleanup EXIT
trap 'echo "FAIL: ${BASH_SOURCE[0]}:$LINENO" >&2' ERR
trap 'exit 130' INT
trap 'exit 143' TERM
source "$ROOT/tests/e2e/helpers.sh"
platform_guard
no_other_consumers
bash "$ROOT/scripts/images.sh" verify
k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
k -n ai-system rollout status daemonset/fake-gpu-device-plugin --timeout=180s
if [[ "$mode" == baseline ]]; then
  # Reuse the full Day 8 proof, including a unique-image saturated rollout.
  DEMO_EVIDENCE_ROOT="$out/integration" bash "$ROOT/scripts/demo.sh"
fi
ns="$(printf '%s' '{"apiVersion":"v1","kind":"Namespace","metadata":{"generateName":"gpu-day9-"}}' | k create -f - -o jsonpath='{.metadata.name}')"
printf '%s\n' "$ns" > "$out/namespace"
if [[ "$mode" == baseline ]]; then
  source "$ROOT/tests/e2e/baseline.sh"
else
  source "$ROOT/tests/e2e/recovery.sh"
fi
snapshot passed
echo "PASS: Day 9 $mode suite."
