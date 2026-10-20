# Environment inspection — 20 October 2026

Read-only inspection performed before implementation. No tools installed, profiles started/stopped, Kubernetes objects changed, or Git initialization performed.

| Check | Observed result | Consequence |
| --- | --- | --- |
| Target directory | Empty at inspection; no `.git` | Planning documents are the first files; initialize Git only after plan approval |
| Repository instructions | No AGENTS.md found in target or reference trees, or checked ancestor paths `/`, `/Users`, user home, and `Preparation` | Recheck at the start of each implementation day |
| Architecture | `uname -m`: arm64 | Default image platform linux/arm64 |
| CPU | `sysctl -n hw.ncpu`: 10 | Proposed Colima budget: 6 vCPUs |
| Physical memory | `sysctl -n hw.memsize`: 17179869184 bytes (16 GiB) | Proposed Colima budget: 8 GiB; do not assume other VMs can run concurrently |
| Disk | Workspace volume: 460 GiB total, about 375 GiB available | Proposed virtual disk cap: 40 GiB |
| Xcode CLI tools | `/Library/Developer/CommandLineTools` | Compiler prerequisites appear present; verify actual build on Day 1 |
| Git and jq | `/usr/bin/git`, `/usr/bin/jq` | Available; versions still to be recorded by doctor |
| Go, Docker, Buildx, Colima, Minikube, kubectl, Kubebuilder | Commands not found with `/opt/homebrew/bin:/opt/homebrew/sbin` included | Installation/discovery is Day 1 work; do not copy historical reference versions as installed facts |
| Alternative Go paths | `/usr/local/bin/go` and `/usr/local/go/bin/go` absent | No working Go installation identified |
| Existing profiles/contexts | CLI listing attempted but tools unavailable | Unknown; mandatory read-only discovery after tools become available |
| Local configuration | `.colima`, `.minikube/profiles`, `.docker/contexts/meta`, and `.kube/config` exist | Presence does not establish valid/running profiles; do not overwrite configuration |

CPU/RAM inspection initially hit a sandbox restriction and succeeded after approved read-only escalation. Free-memory snapshots are volatile and are not a usable VM allocation guarantee. Recheck memory pressure and running profiles immediately before starting the cluster. The actual workload has not been benchmarked.

## Reference inspection

Read `scripts/cluster-up.sh`, `scripts/doctor.sh`, `scripts/install-tools.sh`, `scripts/images.sh`, `Brewfile`, `Makefile`, and `README.md` under `/Users/dhirajsriharsha/Preparation/ProjectMsgQueue`. No files there were modified.

Reusable patterns:

- Named Colima/Docker/Minikube environments; Docker driver with containerd in Minikube.
- Explicit Docker and kubectl contexts; shell script root discovery and Homebrew PATH.
- Separate image build/load operations and ARM64/AMD64 build selection.
- Makefile entry points and environment version evidence under `work/`.

Adaptations needed:

- Reference profile is `gpu-telemetry`; new profile must be `gpu-node-selector`.
- Reference cluster is one node with hard-coded resources. New scripts need three nodes, overrides, role discovery, and topology validation.
- Reference `cluster-down` directly stops Minikube and Colima in Makefile; **no `scripts/cluster-down.sh` exists**. Use a guarded new script or equivalently guarded recipe.
- Reference doctor uses `set -uo pipefail`; new scripts must use `set -euo pipefail` with explicit missing-tool aggregation.
- Drop Helm from required tools. Add Kubebuilder and pinned local controller-gen, Kustomize, setup-envtest.
- Reference README's historical version claims are documentation, not current host observations or a compatibility guarantee.
- Avoid installer-wide upgrades; inspect existing tools and install missing prerequisites deliberately.

## Proposed initial resource settings

| Variable | Proposed default |
| --- | --- |
| COLIMA_PROFILE | gpu-node-selector |
| DOCKER_CONTEXT | colima-gpu-node-selector |
| MINIKUBE_PROFILE / KUBE_CONTEXT | gpu-node-selector |
| PLATFORM_NAMESPACE / WORKLOAD_NAMESPACE | ai-system / ai-workloads |
| COLIMA_CPUS / COLIMA_MEMORY_GIB / COLIMA_DISK_GIB | 6 / 8 / 40 |
| MINIKUBE_NODES | 3 |
| MINIKUBE_CPUS / MINIKUBE_MEMORY_MIB | 2 / 2048 per node, subject to installed-driver verification |
| MINIKUBE_DISK_SIZE | 20g requested; document if Docker driver ignores this and enforce budget at VM level |
| PLATFORM | linux/arm64 |
| FAKE_GPU_COUNT | 2 |

Three 2 GiB node budgets live **inside** the 8 GiB VM; they are not an additional 6 GiB host allocation. Verify how the installed Minikube Docker driver applies CPU/memory limits and inspect the actual node containers. VM overhead and image builds can still cause pressure. If this fails, diagnose before increasing the VM; never silently reduce topology or stop the reference profile. Higher node-count overrides require explicit role selection instead of guessing which worker is the GPU worker.
