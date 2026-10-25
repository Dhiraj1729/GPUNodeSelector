#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
cluster
k get --raw=/readyz >/dev/null
[[ -x "$ROOT/bin/manager" ]] || fail 'Manager binary missing; run make build.'
echo "Running the local manager against context $KUBE_CONTEXT. Stop with Ctrl-C."
exec "$ROOT/bin/manager" --context "$KUBE_CONTEXT"
