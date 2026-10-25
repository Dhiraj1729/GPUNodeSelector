#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
owned
[[ "$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')" == "unix://$HOME/.colima/$PROFILE/docker.sock" ]] || fail 'Unexpected Docker endpoint.'
mkdir -p "$ROOT/work/day-04"
image="gpu-node-selector/mock-inference:${IMAGE_TAG:-dev}"
[[ "$(docker --context "$DOCKER_CONTEXT" image inspect "$image" --format '{{.Config.User}}')" == 65532:65532 ]] || fail 'Mock image must run as 65532:65532.'
cid="$(docker --context "$DOCKER_CONTEXT" run -d --read-only --cap-drop ALL --security-opt no-new-privileges -p 127.0.0.1::9090 -e PORT=9090 -e FAKE_GPU_DEVICES=fake-gpu-0 "$image")"
cleanup() { docker --context "$DOCKER_CONTEXT" logs "$cid" > "$ROOT/work/day-04/container.log" 2>&1 || true; docker --context "$DOCKER_CONTEXT" rm -f "$cid" >/dev/null; }
trap cleanup EXIT
address="$(docker --context "$DOCKER_CONTEXT" port "$cid" 9090/tcp)"
wait_for curl --max-time 3 -fsS "http://$address/readyz"
curl --max-time 3 -fsS "http://$address/healthz" | jq -e '.status=="ok"'
curl --max-time 3 -fsS "http://$address/device" | jq -e '.devices=="fake-gpu-0"'
curl --max-time 3 -fsS -H 'Content-Type: application/json' -d '{"prompt":"hello"}' "http://$address/inference" | tee "$ROOT/work/day-04/inference.json" | jq -e '.response=="mock: hello" and .devices=="fake-gpu-0"'
docker --context "$DOCKER_CONTEXT" stop -t 10 "$cid" >/dev/null
[[ "$(docker --context "$DOCKER_CONTEXT" inspect "$cid" --format '{{.State.ExitCode}}')" == 0 ]] || fail 'Container did not shut down cleanly.'
echo 'PASS: non-root read-only container, HTTP endpoints and SIGTERM shutdown.'
