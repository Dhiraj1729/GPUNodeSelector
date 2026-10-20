# ADR 0002: Use a fake GPU device plugin

Status: Accepted for MVP, 20 October 2026.

Context: Laptop development lacks physical GPUs, but should exercise kubelet registration and allocation rather than permanently editing Node status.

Decision: A Go plugin advertises two Healthy `ai.dhiraj.io/gpu` devices on the GPU worker, implements the supported device-plugin protocol and returns FAKE_GPU_DEVICES from Allocate. Kubelet manages allocation lifetime. Unknown or duplicate request IDs are rejected; repeated valid RPCs remain idempotent. Handle socket recreation and clean shutdown.

Consequences: No CUDA/device files/model runtime is required. The mock proves device identity injection, not actual GPU execution. Node status patching is temporary Stage 1 diagnostic mode only, mutually exclusive with the plugin. Plugin socket permissions and restart behavior require real-node tests.
