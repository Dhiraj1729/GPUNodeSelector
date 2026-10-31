#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
action="${1:-}"
evidence="$ROOT/work/day-07"
mkdir -p "$evidence"
name=fake-gpu-device-plugin
plugin_owned() {
  k -n ai-system get daemonset "$name" -o json | jq -e '.metadata.labels["app.kubernetes.io/part-of"]=="gpu-node-selector"' >/dev/null || fail 'Missing or foreign plugin DaemonSet.'
}
capacity() {
  k get nodes -o json | jq -e --arg r "$RESOURCE" --arg gpu "$GPU_NODE" 'all(.items[]; if .metadata.name==$gpu then .status.capacity[$r]=="2" and .status.allocatable[$r]=="2" else .status.capacity[$r]==null and .status.allocatable[$r]==null end)' >/dev/null
}
case "$action" in
  build)
    cluster
    platform="${PLATFORM:-linux/arm64}"
    case "$platform" in linux/arm64|linux/amd64) ;; *) fail 'Unsupported PLATFORM.' ;; esac
    docker --context "$DOCKER_CONTEXT" build --platform "$platform" --target fake-device-plugin -f "$ROOT/deploy/docker/Dockerfile" -t gpu-node-selector/fake-device-plugin:dev "$ROOT"
    ;;
  load)
    topology
    for image in gpu-node-selector/fake-device-plugin:dev gpu-node-selector/mock-inference:dev; do
      arch="$(docker --context "$DOCKER_CONTEXT" image inspect "$image" --format '{{.Architecture}}')"
      k get nodes -o json | jq -e --arg arch "$arch" 'all(.items[]; .status.nodeInfo.architecture==$arch)' >/dev/null || fail 'Image architecture does not match cluster.'
      minikube --profile "$PROFILE" image load "$image"
    done
    ;;
  deploy)
    topology
    [[ "$(k -n ai-system get daemonset "$name" --ignore-not-found -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}')" != Helm ]] || fail 'Plugin is Helm-managed; use make helm-install.'
    k get pods -A -o json | jq -e 'all(.items[]; .metadata.namespace=="ai-system" and .metadata.labels.app=="fake-gpu-device-plugin" or all(.spec.volumes[]?; (.hostPath.path // "" | startswith("/var/lib/kubelet/device-plugins"))|not))' >/dev/null || fail 'Another device-plugin socket mount exists.'
    existing="$(k -n ai-system get daemonset "$name" --ignore-not-found -o name)"
    if [[ -n "$existing" ]]; then
      plugin_owned
    else
      k get nodes -o json | jq -e --arg r "$RESOURCE" 'all(.items[]; .status.capacity[$r]==null and .status.allocatable[$r]==null)' >/dev/null || fail 'Remove diagnostic GPU capacity before first plugin deployment (see docs/fake-device-plugin.md).'
    fi
    k get nodes -l ai.dhiraj.io/gpu-node=true -o json | jq -e --arg gpu "$GPU_NODE" '.items|length==1 and .[0].metadata.name==$gpu and .[0].metadata.labels["ai.dhiraj.io/gpu-product"]=="A100"' >/dev/null || fail 'Expected exactly one A100 GPU worker.'
    if [[ -z "$(k get namespace ai-system --ignore-not-found -o name)" ]]; then k create namespace ai-system; fi
    k apply -f "$ROOT/config/fake-device-plugin/daemonset.yaml"
    k -n ai-system rollout status daemonset/"$name" --timeout=180s
    wait_for capacity
    ;;
  restart)
    topology
    plugin_owned
    k -n ai-system rollout restart daemonset/"$name"
    k -n ai-system rollout status daemonset/"$name" --timeout=180s
    wait_for capacity
    ;;
  verify)
    topology
    plugin_owned
    k -n ai-system rollout status daemonset/"$name" --timeout=180s
    wait_for capacity
    uid="$(k -n ai-system get daemonset "$name" -o jsonpath='{.metadata.uid}')"
    k -n ai-system get pods -l app="$name" -o json | jq -e --arg gpu "$GPU_NODE" --arg uid "$uid" '[.items[]|select(.metadata.deletionTimestamp==null)] | length==1 and all(.[]; .spec.nodeName==$gpu and any(.metadata.ownerReferences[]; .uid==$uid and .controller==true) and any(.status.conditions[]; .type=="Ready" and .status=="True"))' >/dev/null || fail 'Plugin placement/ownership/readiness mismatch.'
    k get nodes -o json > "$evidence/nodes.json"
    # Server-generated namespace makes cleanup exclusively ours.
    ns="$(printf '%s' '{"apiVersion":"v1","kind":"Namespace","metadata":{"generateName":"gpu-day7-proof-"}}' | k create -f - -o jsonpath='{.metadata.name}')"
    cleanup() {
      k -n "$ns" get pods -o yaml > "$evidence/proof-pods.yaml" 2>&1 || true
      k -n "$ns" get events -o yaml > "$evidence/proof-events.yaml" 2>&1 || true
      k -n ai-system logs -l app="$name" --tail=100 > "$evidence/plugin.log" 2>&1 || true
      k delete namespace "$ns" --wait=true --timeout=90s
    }
    trap cleanup EXIT
    k -n "$ns" create -f "$ROOT/config/fake-device-plugin/proof-pod.yaml"
    k -n "$ns" wait --for=condition=Ready pod/proof --timeout=180s
    [[ "$(k -n "$ns" get pod proof -o jsonpath='{.spec.nodeName}')" == "$GPU_NODE" ]] || fail 'Proof pod is on the wrong worker.'
    k get --raw "/api/v1/namespaces/$ns/pods/proof:8080/proxy/device" | tee "$evidence/device.json" | jq -e '.devices=="fake-gpu-0" or .devices=="fake-gpu-1"'
    capacity
    echo 'PASS: only GPU worker advertises two devices; kubelet injected a valid device ID. Proof namespace will be deleted.'
    ;;
  *) fail 'Usage: plugin.sh build|load|deploy|restart|verify' ;;
esac
