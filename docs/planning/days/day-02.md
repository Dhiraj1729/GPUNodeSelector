# Day 02 — Cluster and scheduling proof

**Complete — remaining lifecycle acceptance closed on 30 October 2026 during Day 10.** Verified running-to-stopped and repeat stop, persisted data after restart, full stopped-VM destruction including runtime data disk, repeated destroy, fresh three-node provisioning/configuration, and the original scheduling proof again before plugin installation. Evidence: work/day-10/lifecycle.log, destroy-stopped-retry.log, destroy-repeat.log, interrupted-cleanup-retry.log, fresh-resume.log and fresh-scheduling-proof.log. The proof removed its namespace and diagnostic GPU capacity. The notes below preserve the original implementation history; their pending statements describe the state at that time.

Startup fix, 24 October 2026: existing profile validation no longer requires a kubeconfig context before Minikube can recreate it. Missing/null context lists are handled explicitly; a conflicting named context still fails before startup. Full context validation runs after start and remains mandatory for other cluster operations. Context regression tests, shell syntax and Git whitespace checks passed. Live `make cluster-up` exited 0, recreated the context, and verified all three nodes Ready. Cluster left running. Evidence: work/day-02/cluster-up-context-fix.log. Minikube reported registry connectivity warnings during startup; cached components allowed startup to finish, but fresh image pulls may need network troubleshooting.

Shutdown fix, 22 October 2026: Minikube multi-node JSON is an array. The shutdown verifier now checks the full expected node-name inventory and every Host state, rejecting empty/partial/duplicate or malformed results. Colima inventory must be readable; permission/errors are not treated as stopped. Already-stopped/absent project VM exits successfully without restarting it. Regression fixtures and shell syntax/whitespace checks passed. `make cluster-down` ran twice successfully against the already-stopped project VM. A new live running-to-stopped cycle was not performed for this fix. Profiles and data remain preserved.

Original status on 22 October 2026: scripts implemented; runtime acceptance pending. Revised user prompt authorized Day 2 and a guarded destroy command. Superseded by the completed acceptance above.

Runtime update: all three nodes reached Ready; worker configuration passed after correcting the Minikube runtime JSON field. Scheduling proof reported PASS for placement, isolation, exhaustion and release. Proof namespace deletion executed. Stop/destroy/fresh rebuild remain unverified; cluster is left running. Logs: work/day-02/configure.log and proof.log.

Added environment helpers, create/configure/stop/destroy scripts, diagnostic capacity operations and an executable scheduling proof that generates Pod manifests as JSON and saves observations. Shell syntax, wrong-context refusal and Git whitespace checks passed. Runtime acceptance is not yet established; consult work/day-02/cluster-up.log for the startup attempt. Do not mark this day complete until proof, cleanup, stop, destroy and fresh rebuild have passed on the host.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 2), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-01.md and verify Day 1 exit criteria are actually complete.

## Scope

Create three isolated nodes, label/taint validated workers, prove manual extended-resource scheduling, and remove diagnostic capacity.

The matching Day 2 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 2 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Final acceptance handoff

- Approval: Day 2 implementation and guarded destroy on 22 October; final lifecycle verification and complete cleanup during approved Day 10.
- Files: scripts/cluster-{up,configure,down,destroy}.sh, scripts/lib/environment.sh, shutdown/context guards, hack/manual-gpu-capacity.sh and hack/scheduling-proof/run.sh, Makefile and local-cluster guide. Day 10 strengthens destroy guards and data-disk deletion.
- Results: placement, taint isolation, two running GPU Pods plus one excess Pending, release/reuse and diagnostic cleanup passed on the fresh cluster. Shutdown, restart persistence, destruction, repeat deletion and fresh rebuild passed. Fixed CPU settings honored.
- Evidence: work/day-02 preserves earlier runs; the Day 10 files listed above close the outstanding checks. Resolved parsing/context/cleanup issues are documented in the daily handoffs.
- No remaining Day 2 blocker. Final production-style local flow uses the plugin and must not run this manual diagnostic while the plugin exists. Follow the Day 10 handoff for final cluster state and source diff.
