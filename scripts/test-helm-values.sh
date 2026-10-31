#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/lib"
cp "$root/scripts/helm.sh" "$tmp/helm.sh"
# Run the real command dispatch with inert cluster and Helm boundaries.
cat > "$tmp/lib/environment.sh" <<'STUB'
ROOT="$HELM_TEST_ROOT"
KUBE_CONTEXT=gpu-node-selector
fail() { echo "ERROR: $*" >&2; exit 1; }
topology() { :; }
k() { :; }
helm() { printf '%s\n' "$@" >> "$HELM_TEST_LOG"; }
STUB
export HELM_TEST_ROOT="$root" HELM_TEST_LOG="$tmp/actual"
values_file="$tmp/custom values.yaml"
printf 'replicas: 1\n' > "$values_file"
for mode in unset empty custom; do
  : > "$HELM_TEST_LOG"
  case "$mode" in
    unset) (unset HELM_VALUES; /bin/bash "$tmp/helm.sh" aiworkloads) ;;
    empty) HELM_VALUES='' /bin/bash "$tmp/helm.sh" aiworkloads ;;
    custom) HELM_VALUES="$values_file" /bin/bash "$tmp/helm.sh" aiworkloads ;;
  esac
  {
    printf '%s\n' --kube-context gpu-node-selector status gpu-node-selector -n ai-system
    printf '%s\n' --kube-context gpu-node-selector upgrade --install mock-llama "$root/charts/aiworkloads" -n ai-workloads --create-namespace --wait --timeout 180s
    if [[ "$mode" == custom ]]; then printf '%s\n' -f "$values_file"; fi
  } > "$tmp/expected"
  diff -u "$tmp/expected" "$HELM_TEST_LOG"
done
: > "$HELM_TEST_LOG"
if HELM_VALUES="$tmp/missing.yaml" /bin/bash "$tmp/helm.sh" aiworkloads > "$tmp/error" 2>&1; then
  echo 'FAIL: missing values file was accepted' >&2
  exit 1
fi
[[ ! -s "$HELM_TEST_LOG" ]] || { echo 'FAIL: Helm called with missing values file'; exit 1; }
grep -q 'HELM_VALUES must name an existing values file' "$tmp/error"
echo 'PASS: Bash 3.2 Helm arguments with unset, empty, spaced-path and missing values overrides.'
