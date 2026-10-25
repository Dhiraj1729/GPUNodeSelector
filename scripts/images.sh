#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
topology
action="${1:-}"
tag="${IMAGE_TAG:-dev}"
[[ "$tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]*$ ]] || fail 'Invalid IMAGE_TAG.'
components="${COMPONENTS:-operator fake-device-plugin mock-inference}"
for component in $components; do
  case "$component" in operator|fake-device-plugin|mock-inference) ;; *) fail 'Unknown image component.' ;; esac
done
native="$(k get node "$GPU_NODE" -o jsonpath='{.status.nodeInfo.architecture}')"
platform="${PLATFORM:-linux/$native}"
case "$platform" in linux/arm64|linux/amd64) ;; *) fail 'Unsupported PLATFORM.' ;; esac
if [[ "$action" == build && "$platform" != "linux/$native" && "$tag" == dev ]]; then fail 'Use a separate IMAGE_TAG for cross-builds.'; fi
mkdir -p "$ROOT/work/day-08/images"
case "$action" in
  build)
    for component in $components; do
      docker --context "$DOCKER_CONTEXT" build --platform "$platform" --build-arg "BUILD_VERSION=$tag" --target "$component" -f "$ROOT/deploy/docker/Dockerfile" -t "gpu-node-selector/$component:$tag" "$ROOT"
    done
    ;;
  load|verify)
    archive="$(mktemp "$ROOT/work/day-08/images/archive.XXXXXX")"
    trap 'rm -f "$archive"' EXIT
    for component in $components; do
      image="gpu-node-selector/$component:$tag"
      arch="$(docker --context "$DOCKER_CONTEXT" image inspect "$image" --format '{{.Architecture}}')"
      k get nodes -o json | jq -e --arg arch "$arch" 'all(.items[]; .status.nodeInfo.architecture==$arch)' >/dev/null || fail "Image architecture mismatch: $image"
      # Docker 29's Id may be an OCI index digest; CRI reports the config digest.
      # Hash the actual config from a local export instead of comparing unlike IDs.
      docker --context "$DOCKER_CONTEXT" image save -o "$archive" "$image"
      config="$(tar -xOf "$archive" manifest.json | jq -er '.[0].Config')"
      tar -xOf "$archive" "$config" | shasum -a 256 | awk '{print "sha256:" $1}' > "$ROOT/work/day-08/images/$component-$tag.digest"
      if [[ "$action" == load ]]; then minikube --profile "$PROFILE" image load --daemon --overwrite "$image"; fi
    done
    for node in $(k get nodes -o jsonpath='{.items[*].metadata.name}'); do
      minikube --profile "$PROFILE" ssh --node "$node" -- sudo crictl images -o json > "$ROOT/work/day-08/images/$node.json"
      for component in $components; do
        image="gpu-node-selector/$component:$tag"
        digest="$(cat "$ROOT/work/day-08/images/$component-$tag.digest")"
        jq -e --arg image "$image" --arg digest "$digest" 'any(.images[]; .id==$digest and any(.repoTags[]?; .==$image or .==("docker.io/"+$image)))' "$ROOT/work/day-08/images/$node.json" >/dev/null || fail "$image missing or stale in $node containerd"
      done
    done
    ;;
  *) fail 'Usage: images.sh build|load|verify' ;;
esac
