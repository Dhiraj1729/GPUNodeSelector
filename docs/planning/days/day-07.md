# Day 07 — Fake device plugin

Status: **Complete, 25 October 2026**. User explicitly approved Day 7 implementation and requested startup-to-verification steps.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 7), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-06.md and verify Day 6 exit criteria are actually complete.

## Scope

Implement inventory, Allocate validation, kubelet registration/ListAndWatch, socket lifecycle recovery and focused tests; prove plugin capacity.

The matching Day 7 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 7 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Delivered

- cmd/fake-device-plugin/main.go and internal/fakegpu/: immutable inventory, options/ListAndWatch/Allocate, count validation, Unix gRPC readiness-before-registration, retries, socket identity recovery, cancellation and process lock.
- internal/fakegpu/plugin_test.go: valid/multiple allocations, empty/unknown/duplicate IDs, repeated RPCs, inventory, canceled stream, transient registration rejection, socket recreation, shutdown cleanup and file/concurrent-process safety.
- config/fake-device-plugin/daemonset.yaml and proof-pod.yaml; plugin Dockerfile stage and bounded Docker build context; build/load/deploy/verify/restart Make targets and scripts/plugin.sh.
- go.mod/go.sum pin kubelet v0.37.0 and gRPC v1.82.1. No unrelated Homebrew upgrades.
- docs/fake-device-plugin.md contains ordered commands, expected results, manual-mode guards, permissions, cleanup and limitations. README, toolchain and planning handoffs updated.

## Decisions

Use installed kubelet v1beta1 generated API aligned with project Kubernetes v1.37.0. Stable IDs, no allocation ledger, repeated RPCs allowed. Unsupported optional methods remain Unimplemented and options advertise false. Device count defaults to 2 (validated 1..256); DaemonSet fixes 2.

Poll socket identity every second with three-second registration deadlines and one-to-eight-second retry backoff. A retained lock file prevents overlapping instances. Only the plugin socket is removed, with inode checks on shutdown; kubelet.sock is never removed. Test harness simulates socket recreation; live restart replaces the DaemonSet Pod.

UID/GID 0 is required to write the root-owned kubelet directory in this local deployment. Live proof succeeded with no privileged mode, all capabilities dropped, no privilege escalation, RuntimeDefault seccomp and read-only root filesystem. No service account token or RBAC. Socket mount is confined to the GPU-worker DaemonSet.

## Actual checks and evidence

All evidence is under ignored work/day-07/:

- unit.log and checks.log preserve initial compile failure: this generated API spells the allocation field DevicesIds, not the older DevicesIDs spelling. Corrected and rerun.
- checks-fixed.log: format/vet/unit/build/race passed, using the existing macOS 15.4 SDK override.
- lifecycle-final.log: strengthened registration-retry and shutdown-cleanup tests passed under race detection.
- regression.log: generation, all admission/reconciliation/status envtest and generated reproducibility passed.
- cluster-up.log: existing project profiles resumed; all three nodes Ready with Colima 6 CPUs and Minikube 2 CPUs/node. Initial read-only Minikube inventory could not query the default Docker socket while Colima was stopped; guarded project-context startup succeeded.
- images.log: cluster labels/taint configured and native linux/arm64 mock/plugin images built.
- live.log: image load, deployment, capacity and proof-Pod allocation passed; namespace gpu-day7-proof-c4kkq deleted.
- restart-proof.log: DaemonSet restart and repeat allocation proof passed; namespace gpu-day7-proof-pzhqj deleted.
- nodes.json, device.json, plugin.log, proof-pods.yaml, proof-events.yaml: last verification evidence; device.json contains fake-gpu-0, no environment value was supplied in the proof manifest.
- final-pods.txt, final-namespaces.txt, daemonset.yaml, dependency-versions.txt: final state and versions. Only the project plugin remains beyond system Pods; no proof namespaces remain.

Shell syntax and Git whitespace checks passed. No manual resource patch was needed: first deployment's absence guard passed. Capacity and allocatable remained 2 during allocation, as expected.

## Handoff

Day 7 exit criteria passed. Cluster remains running with one plugin Pod on gpu-node-selector-m03; operator manager is not deployed. Temporary proof workloads are removed. Earlier source/planning changes and the read-only reference project were preserved. No applicable AGENTS.md found. Work is an uncommitted reviewable diff; no commit or push.

Not claimed: real kubelet service restart, AMD64 image/runtime verification, full AIWorkload/inference integration or Day 2 destroy/rebuild acceptance. These do not replace the focused plugin restart and allocation checks completed here. Next: Day 8 integration after user instruction.
