# Day 08 — Integrated workload path

Status: **Complete — 25 October 2026**. Explicitly approved in chat with the supplied implementation prompt. Stop at the Day 8 handoff.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 8), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-07.md and verify Day 7 exit criteria are actually complete.

## Scope

Build/load/deploy all images, prove per-pod device IDs, 2-to-3-to-1 replicas, reuse, HTTP responses and saturated image rollout.

The matching Day 8 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 8 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Completed implementation

- Packaging: .dockerignore, deploy/docker/Dockerfile, config/manager/, config/rbac/kustomization.yaml, config/platform/, config/default/, config/fake-device-plugin/.
- Automation: Makefile, scripts/images.sh, scripts/deploy.sh, scripts/demo.sh, scripts/install-crd.sh, scripts/lib/project-owned.jq, scripts/lib/crd-owned.jq, scripts/test-platform-ownership.sh.
- Documentation: README.md, docs/demo.md, plugin/mock/reconciliation guides, planning progress/context and this handoff.

The operator runs non-root with a read-only root filesystem, probes, requests, its ServiceAccount and generated RBAC. One replica with Recreate avoids overlapping managers. Guarded deployment refuses foreign resources; CRD installation supports an exact legacy-schema migration. All three native images are loaded and their tags/config digests verified on every node. Identical redeployment preserves the operator Pod UID. Inference rollout uses a unique tag and distinct image metadata.

## Decisions and scope refinements

- Smoke uses generated ai-workloads-day8-* namespaces, cleaned on exit, rather than adopting a user's namespace. Platform remains installed.
- Actual exhaustion contains GPU shortage plus node-selector mismatch. Correct status is Pending/MultipleSchedulingConstraints, CapacityAvailable=Unknown. Only exclusively GPU-shortage clauses require CapacityBlocked/InsufficientGPU.
- Zero surge permits a replacement Pod object while the old Pod terminates. The proof checks at most one nonterminating first Pod, eventual replacement, and the second Pod continuously Ready.
- Fixed CPU/VM budgets unchanged. No manual Node capacity patches, real GPU computation or reference-project edits. AMD64 cross-builds use a separate tag and are not loaded into ARM64 nodes.

## Verification and evidence

All Day 8 exit criteria passed. Evidence is under ignored work/day-08/:

| Command / proof | Result and evidence |
| --- | --- |
| make cluster-up plugin-verify | Passed; preflight.log |
| make docker-build minikube-load install deploy | Native builds/load/install succeeded; initial deploy guard corrected as below; platform.log, final-deploy.log |
| make deploy demo-smoke; repeated deploy/demo/plugin verification | Passed twice; smoke.log, repeat.log; operator-before.txt and operator-after.txt have identical UID |
| make demo-smoke after final digest fix | Passed; final-smoke.log, 20261025T084122Z-8927/ |
| SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk make generate manifests fmt vet test build envtest check-generated race | Passed; regression.log; retained host SDK workaround |
| PLATFORM=linux/amd64 IMAGE_TAG=day8-amd64 make docker-build | All three cross-builds passed; amd64-build.log; runtime not tested |
| Repeated make install and bash scripts/images.sh verify | Passed; install-rerun.log, image-digests-fixed.log |
| Ownership fixtures, shell syntax, manifest rendering and git diff --check | Passed; ownership-tests.log, final-checks.log |

Smoke proves two distinct device IDs, GPU placement and requests/limits, 2→3→1 replicas, truthful exhaustion, capacity reuse, health/readiness/device/inference HTTP, port 9090 update, preserved Service fields and saturated unique-image rollout. Snapshots, scheduler messages/events, per-Pod responses, image inventories, rollout timeline and logs are retained per run. Earlier successful runs: 20261025T083104Z-6928/ and 20261025T083531Z-7769/. All three generated namespaces were deleted.

Resolved failures are retained honestly: initial deployment treated empty resource-query output as invalid ownership. Slurped validation now accepts no objects and rejects foreign objects; fixtures/live reruns passed. Initial image verification compared Docker OCI index digest against CRI config digest (image-digests.log). It now hashes the saved image config; final verification and smoke passed.

## Handoff

No active Day 8 blocker. Cluster remains running with operator/plugin and no smoke/proof namespaces. Day 2 historical lifecycle acceptance remains separately recorded; no real kubelet service restart or AMD64 runtime test is claimed. No commit/push made; reviewable changes preserve earlier work. Proposed checkpoint: `feat: integrate operator plugin and mock workload demo`.

See docs/demo.md for startup and verification commands. Next step: review, then Day 9 end-to-end/recovery work only upon user instruction.
