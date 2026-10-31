# Day 09 — End-to-end and recovery

Status: **Complete — 29 October 2026**. Approved implementation and acceptance/recovery checks passed. Stop at this handoff.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 9), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-08.md and verify Day 8 exit criteria are actually complete.

## Scope

Automate all acceptance cases, real garbage collection/capacity reuse, drift/stability and scoped failure recovery; run baseline suite twice.

The matching Day 9 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 9 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Implementation

Approved by the user's 29 October request to implement and test the Day 9 schedule. Implemented scripts/e2e.sh, tests/e2e/{helpers,baseline,recovery}.sh and the acceptance matrix in tests/e2e/README.md. Added Makefile e2e/e2e-recovery targets, optional evidence-root routing in scripts/demo.sh, docs/troubleshooting.md and planning updates. The baseline composes the Day 8 integration proof rather than duplicating it. No operator/plugin Go changes or new dependencies were needed.

Dedicated server-generated namespaces, a project-local run lock, explicit context/profile/topology/ownership guards, bounded condition waits, diagnostics-before-cleanup and failure exit codes keep test effects scoped. Tests refuse existing outside GPU consumers. Recovery deliberately restarts the single operator, plugin and only the validated project GPU-worker kubelet. It verifies the installed systemd mechanism and a changed InvocationID plus successful new allocations. CPU settings remain 6 VM / 2 per node.

Coverage: original cases 1–17; CPU placement separately from GPU-taint isolation; independent drift repairs; semantic unchanged-state stability; deleted child recreation; real garbage collection and replacement capacity use; simultaneous CRs; zero replicas; product/count/image/port updates; unsatisfiable product/count; invalid-image diagnosis/recovery; current-generation status; saturated unique-image rollout; foreign-controller Deployment and unowned Service preservation. The approved mixed-constraint refinement remains: raw GPU shortage plus selectors is Pending/MultipleSchedulingConstraints, not forced CapacityBlocked.

## Verification record

- `make cluster-up`: passed; work/day-09/preflight.log.
- `SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk make test envtest race`: passed; regression.log. Existing SDK workaround/linker warnings retained.
- First `make e2e`: failed on a harness JSON-scope error in the old-generation assertion. Captured CR correctly had metadata.generation=2 and status/conditions generation=1. Fixed the assertion to retain the root object; also made the removed-toleration predicate null-safe. Diagnostics and successful cleanup retained in baseline-1.log and 20261029T160926Z-3486-baseline/.
- Corrected `make e2e`: passed; baseline-2.log and 20261029T161152Z-4402-baseline/. All garbage-collection, failure/recovery and collision checks passed; namespace deleted.
- Second consecutive `make e2e`: passed; baseline-3.log and 20261029T161332Z-5492-baseline/. No suite changes between the two successful baseline runs; namespace deleted.
- First `make e2e-recovery`: operator/plugin tests passed, then the kubelet precheck safely refused because native SSH output contained CRLF. No kubelet restart occurred in that attempt. recovery.log and 20261029T161525Z-6576-recovery/ retain diagnostics and successful cleanup. Normalized carriage returns before checking the service state.
- Corrected `make e2e-recovery`: passed; recovery-2.log and 20261029T161755Z-6990-recovery/. Real GPU-worker kubelet restart proven by distinct valid systemd InvocationIDs; existing/new device IDs remained exclusive; a final two-replica CR reused capacity after deletion. Namespace deleted. No optional recovery case was skipped.
- Wrong-context refusal: passed before cluster access; wrong-context.log. Shell syntax and git diff --check passed during implementation.

## Handoff

All Day 9 exit criteria passed, including two consecutive baseline runs, scoped recovery and no test leftovers. Final node/platform/namespace/workload snapshots are in work/day-09/final-*. All three nodes are Ready; operator/plugin are healthy; no generated test namespaces or AIWorkloads remain. Cluster is left running. Final shell/whitespace checks are recorded in final-checks.log. Resolved harness failures above are not hidden or counted as successful runs.

No active Day 9 blocker, commit or push. Changes remain a reviewable diff on the user's Day 8 checkpoint. Suggested checkpoint: `test: cover scheduling lifecycle drift and recovery`. Day 10 final demo/verify/documentation work requires user instruction. Historical Day 2 lifecycle acceptance remains separately recorded; Day 9 did not test profile destruction or stop/start persistence. No real GPU computation or AMD64 runtime validation is claimed.

To reproduce: `make doctor`, `make cluster-up cluster-configure`, `make docker-build minikube-load install deploy`, `make e2e` twice, and `make e2e-recovery`. Run sequentially with no other GPU consumers. For regression use the SDKROOT-prefixed command above. Routine shutdown remains `make cluster-down`.
