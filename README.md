# GPU Node Selector

Day 2 lifecycle and scheduling-proof scripts are now available. See [verification steps](docs/local-cluster.md#day-2-verification). Runtime acceptance remains pending; the scripts have not yet demonstrated a complete successful cluster lifecycle.

A Kubernetes AI workload control-plane project: an AIWorkload operator, a fake GPU device plugin, and a mock inference server running on three local Minikube nodes.

**Status: Day 8 integrated workload path implemented.** The in-cluster operator, fake plugin and mock inference server run together. Automated smoke verifies distinct devices, exhaustion with truthful mixed status, capacity reuse, HTTP/port changes and saturated image rollout. Start with [the Day 8 runbook](docs/demo.md). See also [plugin details](docs/fake-device-plugin.md), [status diagnostics](docs/status-and-failures.md) and [the API contract](docs/api.md).

Start with [the ten-day implementation plan](docs/planning/implementation-plan.md). Each day is a milestone, not a fixed calendar date; allow extra time for environment setup and the device plugin.

- [Original requirements, preserved verbatim](docs/planning/original-project-prompt.txt)
- [Environment inspection and limitations](docs/planning/environment-inspection.md)
- [Technical design, API, protocols, test matrix, and final demo](docs/planning/technical-design.md)
- [Persistent implementation context](docs/planning/project-context.md)
- [Progress and handoff log](docs/planning/progress.md)
- [Daily context files](docs/planning/days/day-01.md)

To resume in a new chat, use this prompt, changing the day number:

> Work in /Users/dhirajsriharsha/Preparation/GpuNodeSelector. Read README.md, docs/planning/project-context.md, docs/planning/progress.md, docs/planning/implementation-plan.md, docs/planning/technical-design.md, and docs/planning/days/day-01.md. Check applicable repository instructions and current changes. Verify that plan approval is recorded or explicitly provided in this chat. Implement only this day's approved scope, verify its exit criteria, and update the progress log and daily handoff with actual evidence and remaining blockers. Do not claim unrun tests passed. Do not modify ProjectMsgQueue or operate on unrelated clusters. Stop at the checkpoint unless I ask to continue.

Available now: tooling, cluster/API/build/test targets, `mock-image`, `mock-smoke`, `race`, local `run`, `docker-build`, `minikube-load`, `install`, `deploy`, `demo-smoke` and plugin helpers. Build/load covers all three images; deploy installs the in-cluster operator and plugin. Do not run a second local manager concurrently. Day 2 manual capacity proof must not run while the plugin is installed. Review the preserved original prompt before publishing the repository.

## Day 1 results

- [Architecture](docs/architecture.md)
- [Future local cluster and fixed CPU settings](docs/local-cluster.md)
- [Exact tool versions and compatibility plan](docs/toolchain.md)
- [Day 1 verification and handoff](docs/planning/days/day-01.md)

Run `make doctor` to check the installed clients. Colima CPUs remain fixed at 6 and Minikube CPUs at 2 per node. The proposed 8 GiB RAM / 40 GiB disk VM budget will be checked again before Day 2 provisioning.
