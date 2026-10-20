# GPU Node Selector

A Kubernetes AI workload control-plane project: an AIWorkload operator, a fake GPU device plugin, and a mock inference server running on three local Minikube nodes.

**Status: Day 1 complete.** Git, client tooling, environment checks and architecture documentation are ready. No Colima VM or Minikube cluster has been started. Application implementation begins in later milestones.

Start with [the ten-day implementation plan](docs/planning/implementation-plan.md). Each day is a milestone, not a fixed calendar date; allow extra time for environment setup and the device plugin.

- [Original requirements, preserved verbatim](docs/planning/original-project-prompt.txt)
- [Environment inspection and limitations](docs/planning/environment-inspection.md)
- [Technical design, API, protocols, test matrix, and final demo](docs/planning/technical-design.md)
- [Persistent implementation context](docs/planning/project-context.md)
- [Progress and handoff log](docs/planning/progress.md)
- [Daily context files](docs/planning/days/day-01.md)

To resume in a new chat, use this prompt, changing the day number:

> Work in /Users/dhirajsriharsha/Preparation/GpuNodeSelector. Read README.md, docs/planning/project-context.md, docs/planning/progress.md, docs/planning/implementation-plan.md, docs/planning/technical-design.md, and docs/planning/days/day-01.md. Check applicable repository instructions and current changes. Verify that plan approval is recorded or explicitly provided in this chat. Implement only this day's approved scope, verify its exit criteria, and update the progress log and daily handoff with actual evidence and remaining blockers. Do not claim unrun tests passed. Do not modify ProjectMsgQueue or operate on unrelated clusters. Stop at the checkpoint unless I ask to continue.

Available now: `make tools` installs missing prerequisites and `make doctor` checks client tools without requiring a cluster. Other commands in planning documents describe future targets. The original prompt is retained locally for context; review it before publishing the repository.

## Day 1 results

- [Architecture](docs/architecture.md)
- [Future local cluster and fixed CPU settings](docs/local-cluster.md)
- [Exact tool versions and compatibility plan](docs/toolchain.md)
- [Day 1 verification and handoff](docs/planning/days/day-01.md)

Run `make doctor` to check the installed clients. Colima CPUs remain fixed at 6 and Minikube CPUs at 2 per node. The proposed 8 GiB RAM / 40 GiB disk VM budget will be checked again before Day 2 provisioning.
