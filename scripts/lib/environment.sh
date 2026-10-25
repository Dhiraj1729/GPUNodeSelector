#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
export PATH="/opt/homebrew/bin:/opt/homebrew/sbin:$PATH"
fail() { echo "ERROR: $*" >&2; exit 1; }
PROFILE=gpu-node-selector
export DOCKER_CONTEXT="${DOCKER_CONTEXT:-colima-gpu-node-selector}"
KUBE_CONTEXT="${KUBE_CONTEXT:-$PROFILE}"
[[ "$DOCKER_CONTEXT" == colima-gpu-node-selector && "$KUBE_CONTEXT" == "$PROFILE" ]] || fail 'Unexpected context; refusing operation.'
[[ "${COLIMA_CPUS:-6}" == 6 && "${MINIKUBE_CPUS:-2}" == 2 ]] || fail 'CPU settings must be 6 and 2.'
KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.37.0}"
RESOURCE=ai.dhiraj.io/gpu
NS=gpu-node-selector-proof
EVIDENCE="$ROOT/work/day-02"
mkdir -p "$EVIDENCE"
MARKER="$EVIDENCE/managed-environment"
k() { kubectl --context "$KUBE_CONTEXT" --request-timeout=20s "$@"; }
owned() { [[ -f "$MARKER" && "$(cat "$MARKER")" == "$ROOT" ]] || fail 'No matching creation record; refusing to take over existing profiles.'; }
context_identity() {
  local mode="${1:-required}" config count
  config="$(k config view -o json)" || fail 'Cannot read Kubernetes configuration.'
  count="$(jq -er --arg p "$PROFILE" '[.contexts[]? | select(.name==$p)] | length' <<<"$config")" || fail 'Invalid Kubernetes configuration.'
  if [[ "$count" == 0 && "$mode" == allow-missing ]]; then
    echo 'Project Kubernetes context is absent; Minikube start will recreate it.'
    return
  fi
  jq -e --arg p "$PROFILE" '[.contexts[]? | select(.name==$p)] | length==1 and all(.[]; .context.cluster==$p and .context.user==$p)' <<<"$config" >/dev/null || fail 'Unexpected or missing Kubernetes context identity.'
}
profile_identity() {
  owned
  local config="$HOME/.minikube/profiles/$PROFILE/config.json"
  [[ -f "$config" ]] || fail 'Project Minikube profile is absent.'
  jq -e '.Name == "gpu-node-selector" and .Driver == "docker" and .KubernetesConfig.ContainerRuntime == "containerd"' "$config" >/dev/null || fail 'Unexpected profile configuration.'
}
cluster() {
  profile_identity
  [[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Unexpected Docker endpoint.'
  context_identity
}
topology() {
  cluster
  NODES="$(k get nodes -o json)"
  jq -e '.items|length==3' <<<"$NODES" >/dev/null || fail 'Expected exactly three nodes.'
  jq -e 'all(.items[]; any(.status.conditions[]; .type=="Ready" and .status=="True"))' <<<"$NODES" >/dev/null || fail 'Nodes are not all Ready.'
  [[ "$(jq '[.items[]|select(.metadata.labels|has("node-role.kubernetes.io/control-plane"))]|length' <<<"$NODES")" == 1 ]] || fail 'Expected one control-plane.'
  WORKERS="$(jq -r '.items[]|select(.metadata.labels|has("node-role.kubernetes.io/control-plane")|not)|.metadata.name' <<<"$NODES" | sort)"
  local prior
  prior="$(jq -r '.items[]|select(.metadata.labels["ai.dhiraj.io/gpu-node"]=="true")|.metadata.name' <<<"$NODES")"
  GPU_NODE="${GPU_NODE:-${prior:-$(tail -n 1 <<<"$WORKERS")}}"
  CPU_NODE="${CPU_NODE:-$(grep -vxF "$GPU_NODE" <<<"$WORKERS") }"
  CPU_NODE="${CPU_NODE% }"
  grep -qxF "$GPU_NODE" <<<"$WORKERS" && grep -qxF "$CPU_NODE" <<<"$WORKERS" && [[ "$GPU_NODE" != "$CPU_NODE" ]] || fail 'Invalid worker selection.'
  [[ -z "$prior" || "$prior" == "$GPU_NODE" ]] || fail 'Existing GPU role conflicts with selection.'
}
wait_for() {
  local deadline=$((SECONDS + ${TIMEOUT_SECONDS:-180}))
  until "$@"; do
    (( SECONDS < deadline )) || fail "Timed out: $*"
    sleep 2
  done
}
