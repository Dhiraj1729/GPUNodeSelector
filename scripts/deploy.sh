#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
topology
[[ "$(k -n ai-system get deployment aiworkload-operator --ignore-not-found -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}')" != Helm ]] || fail 'Platform is Helm-managed; use make helm-install.'
bash "$ROOT/scripts/images.sh" verify
mkdir -p "$ROOT/work/day-08"
rendered="$ROOT/work/day-08/platform.yaml"
kubectl kustomize "$ROOT/config/platform" > "$rendered"
# Preflight the entire platform before mutating any platform resource.
k get -f "$rendered" --ignore-not-found -o json | jq -es -f "$ROOT/scripts/lib/project-owned.jq" >/dev/null || fail 'Foreign same-named platform resource; refusing adoption.'
bash "$ROOT/scripts/plugin.sh" deploy
k apply -f "$rendered"
# A rebuilt dev image must restart the manager even though the tag is unchanged.
digest="$(docker --context "$DOCKER_CONTEXT" image inspect gpu-node-selector/operator:dev --format '{{.Id}}')"
k -n ai-system patch deployment aiworkload-operator --type=merge -p "$(jq -n --arg digest "$digest" '{spec:{template:{metadata:{annotations:{"ai.dhiraj.io/image-id":$digest}}}}}')"
k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
k -n ai-system get deployment,daemonset -o yaml > "$ROOT/work/day-08/platform-state.yaml"
