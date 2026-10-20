# Day 01 — Tooling and architecture

Status: **Complete, 20 October 2026**. Overall plan and Day 1 explicitly approved in the user's implementation prompt. Stop here; no Day 2 operations performed.

## Execution plan followed

Read planning and reference files; inspect workspace/tools/profile locations before installation; record approval and initialize Git; install only missing prerequisites; create strict scripts and docs; exercise doctor success/failure paths; verify and save evidence. No applicable AGENTS.md found in checked workspace/reference trees or ancestors.

## Files delivered

- `.gitignore`, `Brewfile`, `Makefile`.
- `scripts/install-tools.sh`, `scripts/doctor.sh` (executable).
- `docs/architecture.md`, `docs/local-cluster.md`, `docs/toolchain.md`.
- `docs/adr/0001-default-scheduler.md`, `0002-fake-device-plugin.md`, `0003-gpu-node-isolation.md`.
- README and planning approval/progress/context updated. Original prompt unchanged.

## Decisions

Module: `github.com/Dhiraj1729/GpuNodeSelector`. Fixed 6 Colima vCPUs and 2 vCPUs per Minikube node. Proposed 8 GiB RAM/40 GiB disk VM on 10 CPU/16 GiB ARM64 host with ~375 GiB free disk. No runtime feasibility claim; inspect pressure before Day 2.

Installed missing client tools; preserved system Git/jq and working app-provided ripgrep. Brewfile records inventory; installer uses an explicit command-to-formula mapping to avoid installing/upgrading working tools. Homebrew auto-update, install-upgrade and cleanup disabled. Buildx already had a dangling symlink that became valid after installation; no user Docker config was overwritten.

Kubebuilder 4.16.0 inspection-only scaffold under ignored work selected Go >=1.26.0, controller-runtime v0.25.0, controller-gen v0.22.0 and Kustomize v5.8.1. Upstream release go.mod confirms Kubernetes libraries v0.37.0. Installed Go 1.27.1; planned Kubernetes/envtest 1.37.0 matches kubectl minor. Real dependency installation/compilation and envtest assets are Day 3 work.

## Commands and outcomes

| Command/check | Result |
| --- | --- |
| Initial command discovery and doctor | Missing tools detected; expected failure, saved |
| Homebrew inventory before installation | No installed formulas reported; no tool CLI profile listing possible initially |
| Git init | Success after sandbox-required escalation |
| make tools, first run | Installed missing tools; then false Buildx conflict after existing symlink became valid |
| Installer correction + make tools rerun | Pass; retains working tools and checks doctor |
| make doctor | Pass, real installed tools, no mocked success |
| DOCTOR_PATH_ONLY=1 PATH=/usr/bin:/bin /bin/bash scripts/doctor.sh | Expected exit 1 and explicit missing-tool diagnostics |
| bash -n scripts/install-tools.sh scripts/doctor.sh | Pass |
| Colima/Docker/Minikube/kubectl inventory | No Colima instances/Minikube profiles/Kubernetes contexts; Docker default only |
| git diff --check | Pass at final verification |
| Ignore checks | work, bin, tools, coverage and credential paths ignored |

No cluster/daemon start, API scaffold in source, application tests, envtest, image build or runtime tests attempted. The disposable scaffold only inspects generated files with dependency fetching disabled.

## Evidence

Ignored local evidence under `work/day-01/`: `doctor-before.txt`, `install-tools.txt`, `install-tools-rerun.txt`, `doctor-after.txt`, `doctor-final.txt`, `doctor-missing.txt`, `profiles-after.txt`, `host-and-initial-inventory.txt`, `scaffold-inspection.txt`, `scaffold-inspection/`, `controller-runtime-go.mod`, and `verification.txt`. Evidence contains no kubeconfig credentials.

## Handoff

All Day 1 exit criteria passed. No outstanding Day 1 blockers. No commit/push/remote; reviewable working diff left. Reference project untouched. Day 2 must adapt named-profile/bootstrap patterns, implement topology validation and scheduling proof, and implement a guarded project-only stop flow that verifies shutdown. Do not copy the reference's one-node/4-CPU/gpu-telemetry configuration. Confirm Day 2 instruction before starting it.
