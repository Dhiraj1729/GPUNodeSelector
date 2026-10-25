#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
cd "$root"
before="$(shasum -a 256 api/v1alpha1/zz_generated.deepcopy.go config/crd/bases/ai.dhiraj.io_aiworkloads.yaml config/rbac/role.yaml)"
make generate manifests
after="$(shasum -a 256 api/v1alpha1/zz_generated.deepcopy.go config/crd/bases/ai.dhiraj.io_aiworkloads.yaml config/rbac/role.yaml)"
[[ "$before" == "$after" ]] || { echo 'FAIL: generated files were stale; review and retain the regenerated files.' >&2; exit 1; }
echo 'PASS: deepcopy, CRD and RBAC generation are reproducible.'
