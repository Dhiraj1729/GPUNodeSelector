#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
check() {
  local wanted="$1" input="$2" actual=0
  jq -e --arg p p --arg profileDir /profiles/p --arg server https://project:8443 -f "$root/scripts/lib/destroy-contexts.jq" <<<"$input" >/dev/null 2>&1 || actual=1
  [[ "$actual" == "$wanted" ]] || { echo 'FAIL: destroy identity fixture' >&2; exit 1; }
}
check 0 '{}'
check 0 '{"clusters":[{"name":"p","cluster":{"server":"https://127.0.0.1:32771"}}]}'
check 0 '{"contexts":[{"name":"other","context":{"cluster":"other","user":"other"}}]}'
check 0 '{"contexts":[{"name":"p","context":{"cluster":"p","user":"p"}}],"users":[{"name":"p","user":{"client-certificate":"/profiles/p/client.crt","client-key":"/profiles/p/client.key"}}],"clusters":[{"name":"p","cluster":{"server":"https://project:8443"}}]}'
check 1 '{"contexts":[{"name":"p","context":{"cluster":"other","user":"p"}}]}'
check 1 '{"contexts":[{"name":"other","context":{"cluster":"p","user":"other"}}]}'
check 1 '{"contexts":[{"name":"other","context":{"cluster":"other","user":"p"}}]}'
check 1 '{"users":[{"name":"p","user":{"client-certificate":"/foreign/client.crt"}}]}'
check 1 '{"clusters":[{"name":"p","cluster":{"server":"https://foreign:8443"}}]}'
check 1 'malformed'
check 1 'null'
check 1 '{"contexts":[{"name":"p","context":{"cluster":"p","user":"p"}},{"name":"p","context":{"cluster":"p","user":"p"}}]}'
echo 'PASS: destroy rejects shared/conflicting contexts, credentials and endpoints.'
