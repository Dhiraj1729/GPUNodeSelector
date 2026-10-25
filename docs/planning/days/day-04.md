# Day 04 — Builders and mock server

Status: **Complete, 24 October 2026**. User approved the full Day 4 implementation and asked to resume the in-progress verification. Work continued from the existing files.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 4), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-03.md and verify Day 3 exit criteria are actually complete.

## Scope

Build pure desired Deployment/Service functions and mock HTTP server; test GPU translations, identity, probes and zero-surge rollout.

The matching Day 4 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 4 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Delivered

- internal/workload/builders.go and table-driven tests: GPU request/limit equality, product/node selectors, exact taint toleration, CPU/memory requests, immutable identity selectors, OwnerReferences, probes/security and zero-surge rollout.
- internal/mockinference and cmd/mock-inference: CPU-only health/readiness/device/inference endpoints, strict bounded JSON, deterministic response, configurable port and signal-driven shutdown.
- deploy/docker/Dockerfile, .dockerignore, explicit-context mock-image/mock-smoke scripts, Makefile build/format extensions and race target.
- docs/reconciliation.md and docs/mock-inference.md explain contracts, decisions and commands.

## Decisions

Use persisted UID-based selector hashes and full-name annotations. Preserve short Service-safe names, derive bounded names for other CR names; no new API name restriction. Builders construct fresh objects without API calls or input mutation. Service allocated fields remain empty for Day 5 merging. Pure-Go static mock image runs in scratch as UID/GID 65532. No device ID is fabricated by the builder. No controller/device plugin was added.

## Actual checks

- make fmt vet test build envtest check-generated race: passed; all 35 Day 3 admission subtests still pass.
- Focused builder/HTTP/config/lifecycle tests: passed, including race detection.
- Native linux/arm64 image build and runtime smoke: passed all four endpoints, non-root image, read-only/capability-free runtime, custom port and SIGTERM exit 0. Temporary containers removed.
- linux/amd64 cross-build under separate day4-amd64 tag: passed; AMD64 runtime not tested.
- Shell syntax and whitespace verification performed at final handoff.

The initial local socket test was sandbox-blocked and rerun with permission. Its occupied-port setup then exposed a macOS address-family mismatch; the test was corrected to match the production wildcard listener and given a deadline. Stuck test processes were stopped and final full/race checks passed. No host SDK files were changed. Historical logs retain these resolved failures.

## Evidence and final state

work/day-04/final-checks.log, container-verification.log, container-final.log, amd64-build.log, inference.json and container.log. Initial failures are in initial.log, checks.log and race.log.

Colima project daemon remains running; no Kubernetes workloads were installed by Day 4. Native dev and separate AMD64 mock images are present in Docker, not loaded into Minikube. Existing Day 1–3 work preserved. Uncommitted reviewable changes; no push or commit. Day 2 lifecycle acceptance remains as previously recorded. Next is Day 5 reconciliation after user instruction.
