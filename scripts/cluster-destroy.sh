#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
[[ "${CONFIRM:-}" == "$PROFILE" ]] || fail 'Permanent deletion requires CONFIRM=gpu-node-selector.'
owned
[[ ! -d "$ROOT/work/day-09/active" ]] || fail 'An E2E/demo run is active; refusing destruction.'
profile_dir="$HOME/.minikube/profiles/$PROFILE"
server=''
if [[ -d "$profile_dir" ]]; then
  profile_identity
  server="$(jq -er '.Nodes[]|select(.ControlPlane==true)|"https://"+.IP+":"+(.Port|tostring)' "$profile_dir/config.json")"
fi
# Validate every context before the first destructive operation. No raw credentials
# are saved in evidence. Minikube normally removes its own kubeconfig entries.
k config view -o json | jq -e --arg p "$PROFILE" --arg profileDir "$profile_dir" --arg server "$server" -f "$ROOT/scripts/lib/destroy-contexts.jq" >/dev/null || fail 'Conflicting/shared project kubeconfig entries; refusing destruction.'
docker_contexts="$(env -u DOCKER_CONTEXT docker context ls --format '{{.Name}}')"
if grep -qxF "$DOCKER_CONTEXT" <<<"$docker_contexts"; then
  [[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Unexpected Docker endpoint; refusing destruction.'
fi
colima list --json >/dev/null # An inventory error must not mean absent.
if [[ -d "$profile_dir" ]]; then
  minikube delete -p "$PROFILE"
fi
if [[ -d "$HOME/.colima/$PROFILE" || -d "$HOME/.colima/_lima/colima-$PROFILE" || -d "$HOME/.colima/_lima/_disks/colima-$PROFILE" ]]; then
  colima delete "$PROFILE" --force --data
fi
if docker context inspect "$DOCKER_CONTEXT" >/dev/null 2>&1; then
  [[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Refusing unexpected stale Docker context.'
  # Removing the validated project context is intentional even if selected.
  env -u DOCKER_CONTEXT docker context rm --force "$DOCKER_CONTEXT"
fi
if kubectl config get-contexts -o name | grep -qxF "$PROFILE"; then
  k config view -o json | jq -e --arg p "$PROFILE" '.contexts[]|select(.name==$p)|.context.cluster==$p and .context.user==$p' >/dev/null || fail 'Unexpected stale Kubernetes context.'
  kubectl config delete-context "$PROFILE"
fi
for section in clusters users; do
  if k config view -o json | jq -e --arg s "$section" --arg p "$PROFILE" 'any(.[$s][]?; .name==$p)' >/dev/null; then
    kubectl config unset "$section.$PROFILE"
  fi
done
if [[ "$(k config view -o json | jq -r '.["current-context"]//""')" == "$PROFILE" ]]; then
  kubectl config unset current-context
fi
[[ ! -d "$profile_dir" && ! -d "$HOME/.colima/$PROFILE" && ! -d "$HOME/.colima/_lima/colima-$PROFILE" && ! -d "$HOME/.colima/_lima/_disks/colima-$PROFILE" ]] || fail 'Profile/VM/data-disk state remains; cleanup is incomplete.'
colima list --json | jq -se --arg p "$PROFILE" 'all(.[]; .name!=$p)' >/dev/null || fail 'Project VM still listed.'
env -u DOCKER_CONTEXT docker context ls --format '{{.Name}}' | { if grep -qxF "$DOCKER_CONTEXT"; then exit 1; fi; } || fail 'Docker context remains.'
k config view -o json | jq -e --arg p "$PROFILE" 'all(.contexts[]?; .name!=$p) and all(.clusters[]?; .name!=$p) and all(.users[]?; .name!=$p)' >/dev/null || fail 'Kubernetes context entries remain.'
echo 'PASS: project Minikube, Colima VM and runtime data disk destroyed; no project VM CPU/RAM usage remains. Source, host tools, shared caches and work evidence retained.'
