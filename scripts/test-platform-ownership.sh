#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
filter="$root/scripts/lib/project-owned.jq"
printf '' | jq -es -f "$filter" >/dev/null
printf '%s' '{"kind":"List","items":[]}' | jq -es -f "$filter" >/dev/null
printf '%s' '{"metadata":{"labels":{"app.kubernetes.io/part-of":"gpu-node-selector"}}}' | jq -es -f "$filter" >/dev/null
for input in '{"metadata":{}}' '{"kind":"List","items":[{"metadata":{"labels":{"app.kubernetes.io/part-of":"foreign"}}}]}' ; do
  if printf '%s' "$input" | jq -es -f "$filter" >/dev/null; then echo 'FAIL: foreign object accepted' >&2; exit 1; fi
done
echo 'PASS: absent, owned and foreign platform resource checks.'
desired='{"spec":{"group":"ai.dhiraj.io"}}'
legacy="$(jq -n --arg applied "$desired" '{metadata:{annotations:{"kubectl.kubernetes.io/last-applied-configuration":$applied}}}')"
printf '%s' "$legacy" | jq -e --argjson desired "$desired" -f "$root/scripts/lib/crd-owned.jq" >/dev/null
for input in '{"metadata":{}}' '{"metadata":{"labels":{"app.kubernetes.io/part-of":"foreign"}}}' ; do
  if printf '%s' "$input" | jq -e --argjson desired "$desired" -f "$root/scripts/lib/crd-owned.jq" >/dev/null; then echo 'FAIL: foreign CRD accepted' >&2; exit 1; fi
done
if printf '%s' "$legacy" | jq -e --argjson desired '{"spec":{"group":"other"}}' -f "$root/scripts/lib/crd-owned.jq" >/dev/null; then echo 'FAIL: different legacy schema adopted' >&2; exit 1; fi
echo 'PASS: exact legacy CRD migration and foreign CRD rejection.'
