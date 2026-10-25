# CPU-only mock inference server

Build with `make build`; the executable is bin/mock-inference. It needs no GPU, CUDA, model weights or network dependency. Configuration is read once at startup and injected into handlers:

| Environment variable | Default | Meaning |
| --- | --- | --- |
| PORT | 8080 | Integer TCP port from 1 to 65535 |
| FAKE_GPU_DEVICES | empty string | Device identity string to report; no allocation is performed |

## HTTP contract

| Request | Success response |
| --- | --- |
| GET /healthz | 200, `{"status":"ok"}` |
| GET /readyz | 200, `{"status":"ready"}`; 503 during shutdown |
| GET /device | 200, `{"devices":"fake-gpu-0"}` or an empty device string when unset |
| POST /inference | `{"prompt":"hello"}` produces 200, `{"response":"mock: hello","devices":"..."}` |

Responses are JSON. Prompt must be a non-whitespace string. Empty/malformed input, unknown fields, wrong types and multiple JSON values return 400. Bodies over 64 KiB return 413. Wrong methods return 405 with Allow; unknown paths return 404. Output is deterministic and echoes the original prompt with a `mock: ` prefix. `/device` preserves the device string exactly, including comma-separated IDs. A successful response demonstrates only mock execution, not physical GPU use.

The HTTP server bounds header/read/write/idle timeouts and header size. SIGINT/SIGTERM marks readiness false, stops accepting requests, and drains active handlers for up to five seconds. On drain timeout it closes the server and exits with an error. Readiness has no external dependencies; inference requests received after shutdown begins return 503. Invalid PORT or a listen failure produces a nonzero exit.

## Native verification

In terminal A:

```sh
make build
PORT=18080 FAKE_GPU_DEVICES=fake-gpu-0 ./bin/mock-inference
```

In terminal B:

```sh
curl -fsS http://127.0.0.1:18080/healthz
curl -fsS http://127.0.0.1:18080/readyz
curl -fsS http://127.0.0.1:18080/device
curl -fsS -H 'Content-Type: application/json' \
  -d '{"prompt":"hello"}' http://127.0.0.1:18080/inference
```

Press Ctrl-C in terminal A to shut down. Omit FAKE_GPU_DEVICES to run without a device identity.

## Container verification

The project Colima Docker daemon must be running. Build and test using explicit project-context scripts:

```sh
make mock-image
make mock-smoke
PLATFORM=linux/amd64 IMAGE_TAG=day4-amd64 make mock-image
```

The multi-stage Dockerfile uses Go 1.27.1 and a scratch runtime, UID/GID 65532:65532, and a static binary. Kubernetes probes are HTTP probes; there is no shell-based HEALTHCHECK. The build context excludes credentials, work evidence and unrelated repository files.

The smoke test verifies the image user, runs a temporary read-only container with no capabilities, uses PORT=9090 and a random loopback host port, calls all endpoints, stops via SIGTERM, checks exit code 0, and removes only the container ID it created. HTTP waits are bounded. Evidence is saved under work/day-04. Day 8 adds verified Minikube image loading and operator-driven HTTP/device/rollout proof; see docs/demo.md.

Native ARM64 build/runtime and AMD64 cross-build passed. AMD64 runtime was not tested. The dev tag remains ARM64; the separate day4-amd64 tag holds the cross-build.

## Automated checks

```sh
make fmt vet test build envtest check-generated race
go test ./internal/workload/... ./internal/mockinference/...
```

Tests cover unset/single/multiple device identities, success/error HTTP contracts, port parsing, occupied port, readiness, listener closure and cancellation. The occupied-port test binds the same wildcard address family as the executable and has a deadline; its initial IPv4-only form permitted a second IPv6 listener on macOS and was corrected. Race checks passed in this session, superseding Day 3's historical C-toolchain limitation for these two packages.
