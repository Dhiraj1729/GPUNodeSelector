#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
check() {
  local wanted="$1" input="$2" actual=0
  jq -e --argjson expected '["p","p-m02","p-m03"]' -f "$root/scripts/lib/stopped-nodes.jq" <<<"$input" >/dev/null 2>&1 || actual=1
  [[ "$actual" == "$wanted" ]] || { echo "FAIL: $input"; exit 1; }
}
check 0 '[{"Name":"p","Host":"Stopped"},{"Name":"p-m02","Host":"Stopped"},{"Name":"p-m03","Host":"Stopped"}]'
check 1 '[{"Name":"p","Host":"Stopped"},{"Name":"p-m02","Host":"Running"},{"Name":"p-m03","Host":"Stopped"}]'
check 1 '[{"Name":"p","Host":"Stopped"}]'
check 1 '[{"Name":"p","Host":"Stopped"},{"Name":"p","Host":"Stopped"},{"Name":"p-m03","Host":"Stopped"}]'
check 1 '[]'
check 1 'null'
check 1 'invalid-json'
echo 'PASS: complete stopped inventory accepted; running, partial, duplicate and invalid inventories rejected.'
