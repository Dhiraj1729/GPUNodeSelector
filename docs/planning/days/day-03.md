# Day 03 — API and validation

Status: **Complete, 24 October 2026**. User explicitly requested implementation according to the Day 3 plan. No later milestone was implemented.

## Session start

Read ../project-context.md, ../progress.md, ../implementation-plan.md (Day 3), and ../technical-design.md. Inspect applicable repository instructions and current changes. Read day-02.md and verify Day 2 exit criteria are actually complete.

## Scope

Scaffold the namespaced AIWorkload API without overwriting planning docs; verify defaults and rejected inputs through API-server admission.

The matching Day 3 section in implementation-plan.md contains files, implementation steps, tests, commands, observable results, exit criteria, risks and proposed checkpoint commit. Use that section as the complete checklist; do not infer completion from this summary.

## Reusable session prompt

> Implement Day 3 in /Users/dhirajsriharsha/Preparation/GpuNodeSelector under the approved plan. Read the planning context, progress log, technical contract, this daily card and previous handoff first. Check the approval record and current workspace state. Complete today's scope and exit checks, preserve ProjectMsgQueue and unrelated profiles, and update this handoff and progress.md with actual evidence. Stop at the checkpoint unless I ask you to continue.

## Delivered

- Kubebuilder go/v4 PROJECT, Go module/sums, API registration and manager entrypoint without reconcilers.
- AIWorkload spec/defaults/validation, native quantity fields, typed phase and condition constants, status subresource, conditions list-map and printer columns.
- Generated deepcopy and CRD, minimal CRD/default Kustomizations, optional viewer RBAC template and sample CR.
- Preserved existing targets and added generate/manifests/fmt/vet/test/build/envtest/check-generated/install/api-smoke.
- Explicit project identity checks for CRD installation and a live API smoke script with uniquely generated namespace cleanup.
- Unit test and 35 envtest admission subtests; API documentation and updated planning context.

## Decisions

Module and dependency pins match the Day 1 record. Scaffold was generated under work/day-03/scaffold.668sFs and selectively adapted; no user files were overwritten by Kubebuilder. No empty controller was installed. Default deployment includes the CRD only; manager deployment/RBAC waits for reconciliation.

CPU accepts nonnegative decimal cores with at most three fractional places or integer millicores; scientific/binary/fractional-milli forms are explicitly excluded. Memory accepts nonnegative Kubernetes quantities. This makes millicore precision exact without approximate numeric calculations. Wire fractional quantities must be strings. The public API has no extended-resource-name or node-assignment field. Long CR names remain supported; Day 4 must derive bounded identity labels.

CGO_ENABLED defaults to 0 for pure-Go builds. No system SDK changes were made. The initial C-enabled manager link failed due to an installed macOS SDK/linker mismatch; pure-Go build passed. Race checks were not run with the incompatible host C toolchain.

## Verification results

| Check | Actual result |
| --- | --- |
| make generate manifests | Passed with controller-gen v0.22.0 |
| make fmt vet test build | Passed with CGO_ENABLED=0 |
| make envtest | Passed, 35 subtests, Kubernetes 1.37.0 darwin/arm64 assets |
| make check-generated | Passed; second generation identical |
| make cluster-up | Passed; restored missing context and all three nodes Ready |
| make install | Passed; CRD Established |
| make api-smoke | Passed; replicas=1/port=8080 defaults, count=0 rejected, explain captured, temporary namespace deleted |
| Shell syntax / Git whitespace | Checked at final handoff |

Envtest proves API admission and subresource behavior, not scheduler/Deployment/GC behavior. Unit tests verify scheme registration and independent deep copies. Raw API cases cover missing fields, invalid labels, fractional counts, above-capacity requests, zero/default replicas, bounds, CPU precision, malformed/negative quantities, and status isolation/generation semantics.

## Resolved failures and limitations

Initial staged scaffold dependency download was sandbox-blocked; authorized network/cache access completed it. The initial memory CEL sign() call was unavailable and rejected by both API servers; compareTo(quantity('0')) passed. These failed attempts remain in logs. Day 2 destroy/fresh-rebuild acceptance remains separate and was not retroactively marked complete.

## Evidence and next step

Local ignored work/day-03 contains verification.log and envtest.log (initial failures), envtest-final.log, checks-final.log, live-final.log, live-defaults.json, live-invalid.txt, explain.txt, and cluster-up.log. Source includes generated CRD/deepcopy artifacts and dependency sums.

Final state: project cluster running, AIWorkload CRD installed, smoke namespace removed. No operator deployment or workload child objects. Changes left as a reviewable working diff; no commit, push or remote operation. Next milestone is Day 4 desired-state builders and mock inference server, after user instruction.
