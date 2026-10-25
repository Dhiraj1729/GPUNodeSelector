# Persistent project context

## Read first in every implementation session

1. Applicable AGENTS.md/instructions and current workspace/Git status.
2. This file and `progress.md` for actual approval and completion state.
3. `implementation-plan.md` and `technical-design.md` for the intended contract.
4. Today's `days/day-NN.md` and previous day's handoff.
5. `original-project-prompt.txt` if a requirement is unclear. Preserve the original requirements unless the user explicitly changes them; report proposed changes.

## Current authorization

Overall plan and Days 1–8 were explicitly approved in this chat. Day 8 integration passed native live smoke, regression and AMD64 cross-build checks; stop at its handoff. Colima CPU=6 and Minikube CPU=2 per node remain fixed. The project cluster is running with the operator and plugin in ai-system; temporary proof Pods/namespaces are gone. Day 9 requires user instruction. See progress.md for historical Day 2 lifecycle checks and docs/demo.md for the current runbook.

## Fixed boundaries

- Target: `/Users/dhirajsriharsha/Preparation/GpuNodeSelector`.
- Reference: `/Users/dhirajsriharsha/Preparation/ProjectMsgQueue`, read-only.
- Profiles/context: `gpu-node-selector`, Docker `colima-gpu-node-selector`; platform namespace `ai-system`, samples `ai-workloads`.
- Default scheduler selects nodes. Operator never sets nodeName, binds Pods, mutates Nodes, counts/reserves GPUs or caps replicas by capacity.
- One control-plane, one CPU worker, one GPU worker; plugin advertises two `ai.dhiraj.io/gpu` devices.
- GPU labels/taint are bootstrap responsibility. Plugin owns resource advertisement; kubelet owns device allocations/lifetime.
- Manual Node capacity patching is temporary diagnostic-only and mutually exclusive with plugin mode.
- OwnerReferences/GC; no finalizer without an actual external obligation.
- No CUDA, real GPU/LLM, model APIs, telemetry, autoscaling, custom scheduler/DRA, Helm, multi-cluster stack, or other scope additions.
- Explicit contexts and resource identity checks; never modify unrelated profiles. Routine shutdown stops, does not delete.
- Preserve unrelated metadata and allocated Service fields; never silently adopt existing foreign/unowned children.
- Event-driven reconciliation, semantic status comparison, generation-aware status, bounded waits and retries.

## Planning findings and proposals

ARM64 Mac, 10 CPUs, 16 GiB RAM, ~375 GiB free disk at initial inspection. Initially missing tools were installed on Day 1. Project cluster is now provisioned and running: VM 6 vCPUs/8 GiB/40 GiB, three nodes at 2 CPUs/2 GiB each nested within VM. Exact toolchain pins are in docs/toolchain.md. The CRD and single in-cluster reconciler are installed. Day 8 smoke uses isolated generated ai-workloads-day8-* namespaces and cleans them on exit. Do not run a second local manager concurrently.

Implemented rollout policy maxSurge=0/maxUnavailable=1. CPU/memory spec means requests. Day 3 CPU wire validation accepts decimal cores (up to three fractional digits) and integer millicores; memory accepts native nonnegative quantities. See docs/api.md. Mock was completed in Day 4. Day 6 reports generation-aware readiness and conservative scheduling diagnosis; see docs/status-and-failures.md. Device Allocate duplicate validation is per request, never a permanent allocation ledger. envtest cannot prove scheduling or GC. Full proposed decisions and confirmation items live in the main plan.

## End-of-day procedure

- Run the day's exit checks; state actual results and failures, including skipped checks.
- Save diagnostic output under ignored `work/day-NN/`; keep concise durable evidence references in daily handoff.
- Record changed files, important decisions, commands/results, unresolved issues, and the next concrete step.
- Update progress.md; mark complete only when exit criteria pass. Do not advance a blocked day by relabeling its tests optional.
- Make a checkpoint commit only if authorized; otherwise leave a reviewable diff and proposed commit message.
- Do not automatically create another chat, send messages to another chat, publish, or push.
