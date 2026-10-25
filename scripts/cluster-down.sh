#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
owned
colima_state() {
  local inventory
  inventory="$(colima list --json)" || fail 'Cannot read Colima inventory; shutdown is not verified.'
  jq -ser --arg p "$PROFILE" '
    [.[] | select(.name == $p)] |
    if length == 0 then "Absent"
    elif length == 1 then .[0].status
    else error("Ambiguous Colima profile") end' <<<"$inventory"
}
vm_state="$(colima_state)"
case "$vm_state" in
  Stopped|Absent)
    echo "Project Colima VM is $vm_state; no project nodes can be running. State preserved."
    exit 0 ;;
  Running) ;;
  *) fail "Unexpected Colima state: $vm_state; refusing to assume shutdown." ;;
esac
if [[ -d "$HOME/.minikube/profiles/$PROFILE" ]]; then
  cluster
  expected="$(jq -ce '.Name as $p | [.Nodes[] | if .Name == "" then $p else ($p + "-" + .Name) end] | select(length > 0)' "$HOME/.minikube/profiles/$PROFILE/config.json")" || fail 'Cannot identify expected nodes.'
  minikube stop -p "$PROFILE"
  # Minikube may return nonzero when stopped; validate its output, not just exit code.
  result=0
  state="$(minikube status -p "$PROFILE" -o json)" || result=$?
  jq -e --argjson expected "$expected" -f "$ROOT/scripts/lib/stopped-nodes.jq" <<<"$state" >/dev/null || fail "Minikube shutdown not verified (status exit $result); Colima left running for diagnosis."
fi
colima stop "$PROFILE"
[[ "$(colima_state)" == Stopped ]] || fail 'Colima shutdown not verified.'
echo 'PASS: project nodes and Colima stopped; profiles and data preserved.'
