#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
# These guards must fire before any Helm/Kubernetes mutation, even for uninstall.
for action in install aiworkloads verify status aiworkloads-remove uninstall; do
  output="$(KUBE_CONTEXT=unrelated bash "$root/scripts/helm.sh" "$action" 2>&1)" && { echo 'FAIL: accepted foreign context'; exit 1; }
  [[ "$output" == *'Unexpected context; refusing operation.'* ]] || { echo "$output"; exit 1; }
done
output="$(COLIMA_CPUS=4 bash "$root/scripts/helm.sh" install 2>&1)" && { echo 'FAIL: accepted changed CPU budget'; exit 1; }
[[ "$output" == *'CPU settings must be 6 and 2.'* ]] || { echo "$output"; exit 1; }
echo 'PASS: Helm entry points reject foreign contexts and changed CPU budgets.'
# Reinstall may encounter the inactive count from any previous chart setting.
for count in 0 2 4 256; do
  printf '{"items":[{"metadata":{"name":"gpu"},"status":{"capacity":{"gpu":"%s"},"allocatable":{"gpu":"0"}}}]}' "$count" |
    jq -e --arg r gpu --arg gpu gpu -f "$root/scripts/lib/inactive-gpu-capacity.jq" >/dev/null
done
for fixture in \
  '{"items":[{"metadata":{"name":"gpu"},"status":{"capacity":{"gpu":"0"},"allocatable":{"gpu":"1"}}}]}' \
  '{"items":[{"metadata":{"name":"other"},"status":{"capacity":{"gpu":"0"},"allocatable":{"gpu":"0"}}}]}' \
  '{"items":[{"metadata":{"name":"gpu"},"status":{"capacity":{"gpu":"4"},"allocatable":{"gpu":"4"}}}]}' \
  '{"items":[{"metadata":{"name":"other"},"status":{"capacity":{"gpu":"4"},"allocatable":{"gpu":"0"}}}]}' \
  '{"items":[{"metadata":{"name":"gpu"},"status":{"capacity":{"gpu":"invalid"},"allocatable":{"gpu":"0"}}}]}'; do
  if jq -e --arg r gpu --arg gpu gpu -f "$root/scripts/lib/inactive-gpu-capacity.jq" <<<"$fixture" >/dev/null; then
    echo 'FAIL: accepted usable, foreign or malformed GPU capacity'; exit 1
  fi
done
jq -e --arg r gpu --arg gpu gpu -f "$root/scripts/lib/inactive-gpu-capacity.jq" <<< '{"items":[{"metadata":{"name":"gpu"},"status":{}}]}' >/dev/null
echo 'PASS: Helm reinstall accepts absent/inactive capacity and rejects usable or foreign capacity.'
