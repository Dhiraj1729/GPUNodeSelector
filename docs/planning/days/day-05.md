# Day 05 — Reconciliation and drift

Status: **Complete, 24 October 2026**. User explicitly approved Day 5, requested idempotency and verification steps. All Day 5 exit checks passed with the documented SDK override for race tests.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 5), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-04.md and verify Day 4 exit criteria are actually complete.

## Scope

Reconcile owned children repeatedly, preserve allocated/unrelated fields, reject foreign ownership, map Pod changes and repair drift.

The matching Day 5 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 5 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Delivered

- internal/controller/aiworkload_controller.go: fetch/deletion handling, defensive validation, ownership checks, managed-field merging, optimistic patches, stable basic status, child watches and validated Pod owner-chain mapping.
- internal/controller/aiworkload_controller_test.go: conflict propagation/retry, valid/forged/stale Pod mapping and NotFound/deletion behavior.
- tests/envtest/reconciliation_test.go: API-defaulted idempotency, image/scale/port/GPU updates, zero replicas, drift, metadata/Service IP preservation, recreation, ownership collisions, real optimistic conflict and running-manager child watches.
- cmd/main.go: register reconciler and explicit context; scripts/run-manager.sh and make run guard project identity for local execution.
- Generated config/rbac/role.yaml; Makefile generation/race updates; check-generated.sh now includes RBAC.
- README.md, docs/reconciliation.md and planning context/progress updated. Detailed automated and optional live steps are in docs/reconciliation.md.

## Decisions

Use an explicit create-or-patch helper with optimistic resourceVersion locking instead of controllerutil.CreateOrPatch. Return conflicts for normal controller-runtime backoff; no internal sleep or periodic requeue. Repeated unchanged children/status cause no write. Semantic status comparison preserves condition transition times.

Preserve unrelated metadata, non-controller references, Service allocations, sidecars, volumes, unrelated selectors/tolerations and unmanaged Pod/Deployment defaults. Own the entire inference container and GPU-key tolerations; document conflicts with admission injectors. Reject incompatible immutable selectors without deleting objects. No Node permissions, GPU capacity calculations, bindings or finalizers. Finalizers update RBAC permits blockOwnerDeletion owner-reference admission only.

Basic status reports Provisioning/ChildrenReconciled with Ready=False; it does not imply readiness or scheduling success. Day 6 will implement observed workload status. In-cluster operator packaging remains a later milestone. Live verification uses replicas=0, so no plugin/image loading is needed.

## Verification and evidence

Evidence under ignored work/day-05/:

- checks.log: initial sandbox/cache failure during generation; rerun with permission.
- verification.log: unit/vet/build/admission/reconciliation tests passed; controller race link failed on default macOS 27 SDK.
- rbac-race.log: generated role fixed by moving RBAC markers to a standalone package comment; pure-Go resolver tags alone did not resolve SDK linking.
- race-sdk.log: race tests passed using the already-installed macOS 15.4 SDK; linker version warnings retained. No host tools changed.
- final-checks.log: generation, formatting, vet, unit tests, build, admission and reconciliation envtest, generated reproducibility and race checks all passed (exit 0). Shell syntax and Git whitespace checks also passed.

```sh
SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk make generate manifests fmt vet test build envtest check-generated race
bash -n scripts/run-manager.sh scripts/check-generated.sh
git diff --check
```

## Handoff boundary

No project cluster start, workload deployment, image build or live GC/scheduling verification was performed during Day 5. Temporary envtest servers are shut down by tests. Existing cluster/profile state and the read-only reference project are untouched. Day 2 lifecycle acceptance remains pending as previously recorded. No applicable AGENTS.md was found. Earlier work is preserved in the uncommitted reviewable diff; no push or commit.

Next: Day 6 status and scheduling diagnostics, upon user instruction.
