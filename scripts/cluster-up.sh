#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
colima list
docker context ls
minikube profile list -o json
kubectl config get-contexts
df -h "$ROOT"
[[ "$(sysctl -n hw.ncpu)" -ge 6 && "$(sysctl -n hw.memsize)" -ge 17179869184 ]] || fail 'Host needs at least 6 CPUs and 16 GiB RAM for this budget.'
if [[ ! -f "$MARKER" ]]; then
  [[ ! -d "$HOME/.colima/$PROFILE" && ! -d "$HOME/.minikube/profiles/$PROFILE" ]] || fail 'Existing profiles lack project creation record.'
  ! docker context inspect "$DOCKER_CONTEXT" >/dev/null 2>&1 || fail 'Existing Docker context lacks creation record.'
  ! kubectl config get-contexts -o name | grep -qxF "$PROFILE" || fail 'Existing Kubernetes context lacks creation record.'
  printf '%s\n' "$ROOT" > "$MARKER"
fi
owned
if [[ -f "$HOME/.minikube/profiles/$PROFILE/config.json" ]]; then profile_identity; fi
context_identity allow-missing
colima start "$PROFILE" --runtime docker --vm-type vz --cpus 6 --memory 8 --disk 40
[[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Unexpected Docker endpoint.'
minikube start -p "$PROFILE" --driver=docker --container-runtime=containerd --nodes=3 --cpus=2 --memory=2048 --kubernetes-version="$KUBERNETES_VERSION"
cluster
k wait --for=condition=Ready nodes --all --timeout=180s
topology
k get nodes -o wide | tee "$EVIDENCE/nodes.txt"
