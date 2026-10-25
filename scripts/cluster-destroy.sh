#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
[[ "${CONFIRM:-}" == "$PROFILE" ]] || fail 'Permanent deletion requires CONFIRM=gpu-node-selector.'
owned
if [[ -d "$HOME/.minikube/profiles/$PROFILE" ]]; then
  cluster
  minikube delete -p "$PROFILE"
fi
if [[ -d "$HOME/.colima/$PROFILE" ]]; then colima delete "$PROFILE" --force; fi
if docker context inspect "$DOCKER_CONTEXT" >/dev/null 2>&1; then
  [[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Refusing unexpected stale Docker context.'
  docker context rm "$DOCKER_CONTEXT"
fi
if kubectl config get-contexts -o name | grep -qxF "$PROFILE"; then
  k config view -o json | jq -e --arg p "$PROFILE" '.contexts[]|select(.name==$p)|.context.cluster==$p and .context.user==$p' >/dev/null || fail 'Unexpected stale Kubernetes context.'
  kubectl config delete-context "$PROFILE"
fi
[[ ! -d "$HOME/.minikube/profiles/$PROFILE" && ! -d "$HOME/.colima/$PROFILE" ]] || fail 'Profile state remains.'
echo 'Project profiles destroyed; source and evidence retained.'
