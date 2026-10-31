#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cd "$ROOT"
out="$ROOT/work/day-10/verify/$(date -u +%Y%m%dT%H%M%SZ)-$$"
mkdir -p "$out"
exec > >(tee "$out/verify.log") 2>&1
trap 'result=$?; printf "%s\n" "$result" > "$out/exit-code"; echo "Verification evidence: $out (exit $result)"' EXIT
make doctor fmt-check vet test build check-generated envtest race shell-test
bash "$ROOT/scripts/audit-platform.sh"
E2E_EVIDENCE_ROOT="$out/e2e" bash "$ROOT/scripts/e2e.sh" baseline
git diff --check
echo 'PASS: format, vet, unit, build, generation, envtest, race, shell guards, platform audit and E2E.'
