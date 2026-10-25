#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
# Stub only the local kubeconfig read; exercise the production validation function.
k() { printf '%s\n' "$fixture"; }
check() {
  local expected="$1" mode="$2" result=0
  fixture="$3"
  (context_identity "$mode") >/dev/null 2>&1 || result=1
  [[ "$result" == "$expected" ]] || fail "Context regression: $mode $fixture"
}
check 0 allow-missing '{"contexts":null}'
check 0 allow-missing '{"contexts":[]}'
check 1 required '{"contexts":null}'
check 1 required '{"contexts":[]}'
check 0 required '{"contexts":[{"name":"gpu-node-selector","context":{"cluster":"gpu-node-selector","user":"gpu-node-selector"}}]}'
check 1 allow-missing '{"contexts":[{"name":"gpu-node-selector","context":{"cluster":"other","user":"other"}}]}'
check 1 required 'malformed'
check 0 allow-missing '{"contexts":[{"name":"unrelated","context":{"cluster":"other","user":"other"}}]}'
echo 'PASS: missing contexts allowed only before startup; conflicting and malformed contexts rejected.'
