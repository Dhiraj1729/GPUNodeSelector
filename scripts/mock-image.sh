#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
owned
[[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Unexpected Docker endpoint.'
platform="${PLATFORM:-linux/arm64}"
case "$platform" in linux/arm64|linux/amd64) ;; *) fail 'PLATFORM must be linux/arm64 or linux/amd64.' ;; esac
docker --context "$DOCKER_CONTEXT" build --platform "$platform" --target mock-inference -f "$ROOT/deploy/docker/Dockerfile" -t "gpu-node-selector/mock-inference:${IMAGE_TAG:-dev}" "$ROOT"
