# Local cluster design

## Day 2 verification

Run these from the repository root. Startup creates an ownership record under work/day-02; keep it for safe reuse and destruction. Existing profiles without this record are rejected rather than adopted. The proof generates standalone Pod manifests, saves exhaustion/placement/events evidence, and automatically removes its namespace and manual capacity on success or failure where the API remains reachable.

```sh
make doctor
make cluster-up
make cluster-configure
make cluster-configure # rerun check
make scheduling-proof
make scheduling-proof-cleanup # retry if automatic cleanup failed
kubectl --context gpu-node-selector get nodes -o wide
make cluster-down
make cluster-up # restart preserved state
make cluster-configure
make cluster-destroy CONFIRM=gpu-node-selector
make cluster-up # fresh rebuild
make cluster-configure
make scheduling-proof
make cluster-down
```

Destroy permanently deletes this project's cluster and VM data. Source, documentation and work evidence are retained. Contexts are removed only after their project identity is checked; ambiguous state causes refusal. Missing confirmation fails before deletion. If cleanup fails, inspect the saved evidence and retry cleanup before continuing. Runtime proof requires image downloads; override PROOF_IMAGE with a reachable multiarchitecture pause image if necessary. A failed download is not scheduling success.

Verification expectations: three Ready nodes; one CPU and one GPU worker; two GPU pods Running and one Unschedulable with Insufficient ai.dhiraj.io/gpu; deleting a running pod admits the previously pending pod; GPU-selected pod without toleration stays unscheduled. Cleanup removes the namespace and manual capacity/allocatable. Stop preserves state; destroy removes both profile directories, and fresh rebuild must repeat the proof.

No Colima VM or Minikube cluster was created or started on Day 1. These are Day 2 settings and safeguards.

| Setting | Value |
| --- | --- |
| Colima profile | gpu-node-selector |
| Docker context | colima-gpu-node-selector |
| Minikube profile / Kubernetes context | gpu-node-selector |
| Platform / sample namespace | ai-system / ai-workloads |
| Colima CPUs | **6 vCPUs, fixed by the user** |
| Minikube CPUs | **2 vCPUs per node, fixed by the user** |
| Colima RAM / disk | proposed 8 GiB / 40 GiB |
| Node count / memory | 3 / proposed 2048 MiB per node |
| Driver / node runtime | Docker / containerd |
| Kubernetes version | planned v1.37.0 |
| Local image architecture | linux/arm64; support linux/amd64 builds later |

The inspected host has 10 logical CPUs, 16 GiB physical RAM and approximately 375 GiB available disk. Six VM vCPUs and an 8 GiB VM memory budget are plausible starting limits, leaving nominal host headroom. Three 2 GiB node budgets are nested within the VM, not another 6 GiB host allocation. CPU limits likewise share the VM's six vCPUs. Node overhead and builds can still create pressure; this is a feasibility assessment, not a successful provisioning test. Recheck memory pressure and existing VMs before Day 2; do not silently alter the fixed CPU settings or stop another project. A 40 GiB virtual disk is feasible against observed free space; check actual usage as images accumulate.

Day 1 CLI inventory after installation: no Colima instances, no valid/invalid Minikube profiles, no Kubernetes contexts; Docker has only its default context. The expected project contexts are deliberately absent. Client doctor succeeds without a running daemon. Future scripts must validate exact project membership before mutations and always pass explicit contexts/profiles instead of relying on global current context.

## Patterns adapted from ProjectMsgQueue on Day 2

The read-only reference is `/Users/dhirajsriharsha/Preparation/ProjectMsgQueue`. Its cluster-up script uses a named Colima profile, Docker through Colima, Minikube Docker driver/containerd, explicit contexts, Ready waits and environment-version evidence. Adapt these patterns with safe repository-root discovery, overrides, error messages and `set -euo pipefail`.

Do **not** copy the reference unchanged: it creates only one Minikube node, assigns **4 Colima CPUs**, and uses **gpu-telemetry** names. This project requires three nodes, fixed **6 Colima CPUs / 2 CPUs per node**, and **gpu-node-selector** names. Reference node memory/CPU values must not leak into new scripts. Reference doctor uses a different shell strictness policy; the new doctor aggregates missing commands explicitly under `set -euo pipefail`.

Discover the control-plane using role labels and require two Ready workers. Select/validate CPU and GPU worker roles, persist role labels for reruns, and reject ambiguous topology. Label GPU worker `ai.dhiraj.io/gpu-node=true` and `ai.dhiraj.io/gpu-product=A100`; taint `ai.dhiraj.io/gpu=true:NoSchedule`. Do not assume the control-plane is tainted by default. Prove CPU-worker placement with a CPU selector and GPU isolation with a separate no-toleration negative test.

The reference Makefile directly calls Minikube stop then Colima stop; it has no cluster-down script. Day 2 will implement a **guarded cluster-down flow** that validates both project identities, stops only this project's Minikube and Colima profiles, and **verifies shutdown**. Stopping preserves profiles; deletion is a separate explicit action. Handle already-stopped/absent state without targeting a default or unrelated profile.

Build/load separation from the reference images script will be retained later. Docker image builds do not populate Minikube's containerd store; load images explicitly on all nodes and verify architecture. No image build/load is performed on Day 1.

Stage 1 uses temporary manual capacity patching for scheduling proof. Require plugin absence, validate node/context, wait for allocatable, and remove diagnostic capacity before Stage 2 plugin deployment. Final demonstration uses the fake plugin exclusively.

## Day 1 commands

```sh
make tools   # missing commands only; no brew upgrade, daemon start or cluster creation
make doctor  # client availability/versions, not cluster health
bash -n scripts/install-tools.sh scripts/doctor.sh
```

The installer preserves working commands (including system Git/jq), disables Homebrew auto-update/install-upgrade/cleanup, installs required dependency packages as needed, and repairs no existing files blindly. A missing Buildx formula can restore an existing symlink; otherwise only an absent plugin entry is linked. Conflicting entries cause an actionable error.
