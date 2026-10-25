#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster
desired="$(k create --dry-run=client -f "$ROOT/config/crd/bases/ai.dhiraj.io_aiworkloads.yaml" -o json)"
existing="$(k get crd aiworkloads.ai.dhiraj.io --ignore-not-found -o json)"
if [[ -n "$existing" ]]; then
  # One-time migration of the exact Day 3 last-applied schema, not arbitrary adoption.
  jq -e --argjson desired "$desired" -f "$ROOT/scripts/lib/crd-owned.jq" <<<"$existing" >/dev/null || fail 'Foreign CRD or unrecognized legacy schema; refusing overwrite.'
fi
jq '.metadata.labels["app.kubernetes.io/part-of"]="gpu-node-selector"' <<<"$desired" | k apply -f -
k wait --for=condition=Established crd/aiworkloads.ai.dhiraj.io --timeout=60s
