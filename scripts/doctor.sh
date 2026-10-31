#!/usr/bin/env bash
set -euo pipefail
# Honor an explicit test PATH without silently finding excluded host tools.
if [[ "${DOCTOR_PATH_ONLY:-0}" != 1 ]]; then
  export PATH="/opt/homebrew/bin:/opt/homebrew/sbin:$PATH"
fi
missing=0
for tool in go git colima docker minikube kubectl kubebuilder jq rg helm make xcode-select; do
  if command -v "$tool" >/dev/null 2>&1; then
    printf 'FOUND %s: %s\n' "$tool" "$(command -v "$tool")"
  else
    printf 'MISSING %s: install prerequisites with make tools.\n' "$tool" >&2
    missing=1
  fi
done
if [[ "$missing" -ne 0 ]]; then
  echo 'FAIL: required commands are missing; no cluster operations attempted.' >&2
  exit 1
fi
check() {
  if ! "$@"; then
    printf 'FAIL: command failed:' >&2
    printf ' %q' "$@" >&2
    printf '\n' >&2
    exit 1
  fi
}
check xcode-select -p
check go version
check git --version
check colima version
check docker --version
check docker buildx version
check minikube version
check kubectl version --client=true
check kubebuilder version
check helm version --short
check jq --version
check rg --version
check make --version
echo 'PASS: Day 1 client tools available. Cluster/daemon readiness is not checked.'
