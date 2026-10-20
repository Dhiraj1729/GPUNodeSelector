# Persistent project context

## Read first in every implementation session

1. Applicable AGENTS.md/instructions and current workspace/Git status.
2. This file and `progress.md` for actual approval and completion state.
3. `implementation-plan.md` and `technical-design.md` for the intended contract.
4. Today's `days/day-NN.md` and previous day's handoff.
5. `original-project-prompt.txt` if a requirement is unclear. Preserve the original requirements unless the user explicitly changes them; report proposed changes.

## Current authorization

Overall plan and Day 1 implementation approved on 20 October 2026. Execute Day 1 without another routine approval, then stop at its handoff. Do not start Colima or Minikube. Colima CPU=6 and Minikube CPU=2 per node are fixed by the user. See progress.md for actual outcomes.

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

ARM64 Mac, 10 CPUs, 16 GiB RAM, ~375 GiB free disk. Initially missing Go/container/Kubernetes commands were installed on Day 1. Profile inventory is now known; consult the handoff. Proposed VM 6 vCPUs/8 GiB/40 GiB, node limits 2 CPUs/2 GiB each nested within VM. Day 1 complete: exact installed versions and chosen scaffold pins are in docs/toolchain.md. No active profiles/contexts except Docker default.

Proposed rollout policy maxSurge=0/maxUnavailable=1. CPU/memory spec means requests. Mock implemented Day 4. Device Allocate duplicate validation is per request, never a permanent allocation ledger. envtest cannot prove scheduling or GC. Full proposed decisions and confirmation items live in the main plan.

## End-of-day procedure

- Run the day's exit checks; state actual results and failures, including skipped checks.
- Save diagnostic output under ignored `work/day-NN/`; keep concise durable evidence references in daily handoff.
- Record changed files, important decisions, commands/results, unresolved issues, and the next concrete step.
- Update progress.md; mark complete only when exit criteria pass. Do not advance a blocked day by relabeling its tests optional.
- Make a checkpoint commit only if authorized; otherwise leave a reviewable diff and proposed commit message.
- Do not automatically create another chat, send messages to another chat, publish, or push.
