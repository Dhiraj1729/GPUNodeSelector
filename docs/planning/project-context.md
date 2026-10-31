# Persistent project context

## Read first in every implementation session

1. Applicable AGENTS.md/instructions and current workspace/Git status.
2. This file and `progress.md` for actual approval and completion state.
3. `implementation-plan.md` and `technical-design.md` for the intended contract.
4. Today's `days/day-NN.md` and previous day's handoff.
5. `original-project-prompt.txt` if a requirement is unclear. Preserve the original requirements unless the user explicitly changes them; report proposed changes.

## Current authorization

Overall plan and Days 1–10 were explicitly approved in this chat. All milestones completed by 30 October 2026, including full verification, demos, recovery, stop/start persistence, fresh provisioning and complete project cleanup. Final state: project Minikube/Colima VM/runtime disk and contexts are destroyed; source, evidence, host tools and shared caches remain. Day 9/10 source changes are uncommitted for review. Colima CPU=6 and Minikube CPU=2 per node remain fixed. Stop at final handoff; new work needs user instruction. See progress.md, days/day-10.md and README.md for evidence and recreation commands.

## Helm extension (31 October 2026)

User authorized platform and mock Llama AIWorkload Helm charts and a project-focused README. Implemented charts, explicit-context Make wrappers, generation-aware workload verification, ordered uninstall, schema/render tests and docs/helm.md. Helm owns AIWorkloads; the operator still owns inference Deployments/Services. `helm-inference` aliases `helm-aiworkloads`. Existing kubectl-managed platform resources require deliberate migration; no forced adoption. Local Helm 4.3.0 chart tests pass. The project Colima profile was observed stopped during this extension and was not started or modified; live Helm acceptance remains to be run. Historical Day 10 destruction describes that earlier handoff, not the current host state. See docs/helm.md for the current deployment interface.

## Fixed boundaries

- Target: `/Users/dhirajsriharsha/Preparation/GpuNodeSelector`.
- Reference: `/Users/dhirajsriharsha/Preparation/ProjectMsgQueue`, read-only.
- Profiles/context: `gpu-node-selector`, Docker `colima-gpu-node-selector`; platform namespace `ai-system`, samples `ai-workloads`.
- Default scheduler selects nodes. Operator never sets nodeName, binds Pods, mutates Nodes, counts/reserves GPUs or caps replicas by capacity.
- One control-plane, one CPU worker, one GPU worker; plugin advertises two `ai.dhiraj.io/gpu` devices.
- GPU labels/taint are bootstrap responsibility. Plugin owns resource advertisement; kubelet owns device allocations/lifetime.
- Manual Node capacity patching is temporary diagnostic-only and mutually exclusive with plugin mode.
- OwnerReferences/GC; no finalizer without an actual external obligation.
- No CUDA, real GPU/LLM, model APIs, telemetry, autoscaling, custom scheduler/DRA, multi-cluster stack, or other scope additions.
- Explicit contexts and resource identity checks; never modify unrelated profiles. Routine shutdown stops, does not delete.
- Preserve unrelated metadata and allocated Service fields; never silently adopt existing foreign/unowned children.
- Event-driven reconciliation, semantic status comparison, generation-aware status, bounded waits and retries.

## Planning findings and proposals

ARM64 Mac, 10 CPUs, 16 GiB RAM, ~375 GiB free disk at initial inspection. Tools were installed on Day 1. The tested environment uses a VM with 6 vCPUs/8 GiB/configured 40 GiB runtime disk and three nested nodes at 2 CPUs/2 GiB each. It was destroyed after final verification to release runtime resources. Recreate and install images/CRD/platform using README.md. Exact toolchain pins are in docs/toolchain.md. Automated tests use generated namespaces and clean them on exit. Do not run a second local manager concurrently with the in-cluster operator.

Implemented rollout policy maxSurge=0/maxUnavailable=1. CPU/memory spec means requests. Day 3 CPU wire validation accepts decimal cores (up to three fractional digits) and integer millicores; memory accepts native nonnegative quantities. See docs/api.md. Mock was completed in Day 4. Day 6 reports generation-aware readiness and conservative scheduling diagnosis; see docs/status-and-failures.md. Device Allocate duplicate validation is per request, never a permanent allocation ledger. envtest cannot prove scheduling or GC. Full proposed decisions and confirmation items live in the main plan.

## End-of-day procedure

- Run the day's exit checks; state actual results and failures, including skipped checks.
- Save diagnostic output under ignored `work/day-NN/`; keep concise durable evidence references in daily handoff.
- Record changed files, important decisions, commands/results, unresolved issues, and the next concrete step.
- Update progress.md; mark complete only when exit criteria pass. Do not advance a blocked day by relabeling its tests optional.
- Make a checkpoint commit only if authorized; otherwise leave a reviewable diff and proposed commit message.
- Do not automatically create another chat, send messages to another chat, publish, or push.
