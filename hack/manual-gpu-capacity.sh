#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/../scripts/lib/environment.sh"
topology
# Conservative exclusion: any device-plugin socket mount blocks manual mode.
k get pods -A -o json | jq -e 'all(.items[]; all(.spec.volumes[]?; (.hostPath.path // "" | startswith("/var/lib/kubelet/device-plugins"))|not))' >/dev/null || fail 'Device-plugin socket mount found; manual mode forbidden.'
capacity() {
  k get node "$GPU_NODE" -o json | jq -e --arg r "$RESOURCE" --arg value "$1" 'if $value=="absent" then (.status.capacity[$r]==null and .status.allocatable[$r]==null) else (.status.capacity[$r]==$value and .status.allocatable[$r]==$value) end' >/dev/null
}
case "${1:-}" in
  apply)
    k patch node "$GPU_NODE" --subresource=status --type=json -p '[{"op":"add","path":"/status/capacity/ai.dhiraj.io~1gpu","value":"2"}]'
    wait_for capacity 2 ;;
  verify) capacity 2 ;;
  remove)
    k get pods -A -o json | jq -e --arg r "$RESOURCE" 'all(.items[]; all(.spec.containers[]; (.resources.limits[$r] // "0")=="0"))' >/dev/null || fail 'GPU consumers remain; remove them first.'
    if k get node "$GPU_NODE" -o json | jq -e --arg r "$RESOURCE" '.status.capacity[$r]!=null' >/dev/null; then
      k patch node "$GPU_NODE" --subresource=status --type=json -p '[{"op":"remove","path":"/status/capacity/ai.dhiraj.io~1gpu"}]'
    fi
    wait_for capacity absent ;;
  *) fail 'Usage: manual-gpu-capacity.sh apply|verify|remove' ;;
esac
