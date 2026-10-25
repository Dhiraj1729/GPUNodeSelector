# Fake GPU device plugin

The plugin advertises two healthy devices, `fake-gpu-0` and `fake-gpu-1`, as `ai.dhiraj.io/gpu` on the GPU worker. These are simulated devices: no CUDA, device files or GPU compute. Kubelet advertises capacity and owns allocation lifetime; the plugin has no permanent reservation map and makes no Kubernetes API calls.

## Protocol and lifecycle

The project pins k8s.io/kubelet v0.37.0 to match its Kubernetes libraries and uses its v1beta1 API with gRPC v1.82.1. The gRPC service starts and passes a local RPC check before registration at kubelet.sock. ListAndWatch sends a fresh inventory and waits until its stream is canceled. Allocate validates the complete request before returning one response per container, with a sorted comma-separated FAKE_GPU_DEVICES environment value. Unknown IDs, empty requests and duplicate IDs within/across containers are rejected. Identical later RPCs succeed.

PreStartContainer and preferred allocation are not advertised; optional calls inherit Unimplemented responses. Device count is configurable from 1 to 256, defaults to 2, and is explicitly 2 in the DaemonSet.

The runtime polls socket identity every second, with registration RPC deadlines of three seconds and retry backoff from one to eight seconds. If kubelet removes the plugin socket, the server restarts and registers again. Kubelet socket replacement triggers re-registration. SIGTERM cancels the process, closes streams and removes only its own socket if its identity still matches. It never deletes kubelet.sock. A persistent lock file prevents overlapping instances; non-socket/symlink collisions at the plugin socket are refused. The lock file is retained after shutdown to avoid inode races.

Reference checked for this implementation: [Kubernetes device plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/). Installed generated API source is the build-time contract.

## Deployment and permissions

The DaemonSet runs only on the labeled A100 GPU worker and tolerates the exact GPU NoSchedule taint. It mounts `/var/lib/kubelet/device-plugins` as a directory, not individual sockets. UID/GID 0 is used for the root-owned kubelet directory; privileged mode is not enabled, all capabilities are dropped, privilege escalation is disabled, seccomp is RuntimeDefault and the image root filesystem is read-only. There is no Kubernetes RBAC or service-account token mount.

Readiness/liveness checks invoke the binary's local gRPC health mode. They establish that the plugin serves, not that registration succeeded. `plugin-verify` separately checks advertised resources and actual allocation. Local images use `:dev` tags and, since Day 8, IfNotPresent policy. Shared image tooling detects the cluster architecture and refuses mismatched loads.

## Verify from cluster startup

```sh
cd /Users/dhirajsriharsha/Preparation/GpuNodeSelector
make doctor
make cluster-up
make cluster-configure
make docker-build
make minikube-load
make plugin-deploy
make plugin-verify
make plugin-restart
make plugin-verify
```

No manager process, CRD installation or AIWorkload is needed for the focused plugin proof. Cluster CPUs remain Colima=6, Minikube=2 per node. Since Day 8, `docker-build` and `minikube-load` cover all three images, with per-node image digest verification. See docs/demo.md for full integration. The scripts validate profile/context identity and refuse foreign DaemonSets or competing socket mounts.

On first deployment, GPU capacity must be absent, including any manual Day 2 diagnostic resource. If deployment refuses manual capacity, first ensure no plugin or GPU consumers remain, then run:

```sh
bash hack/manual-gpu-capacity.sh remove
make plugin-deploy
```

Do not run manual capacity commands after the plugin is deployed. The deployment helper never silently patches Node status. If GPU-consuming Pods already occupy both devices, remove only your known test workloads before running the proof.

Expected verification:

- Exactly one owned Ready plugin Pod on the GPU worker.
- Only that worker advertises capacity=2 and allocatable=2.
- A temporary one-GPU mock Pod becomes Ready on that worker.
- Its `/device` response contains fake-gpu-0 or fake-gpu-1, injected by kubelet's Allocate call (the manifest does not supply it).
- The temporary server-named proof namespace is deleted, including on failure; evidence is retained in work/day-07/.
- Restarting the DaemonSet restores registration; rerunning verification proves a new allocation works.

Capacity/allocatable remain 2 while one device is occupied. They are not free-device counters. Plugin restart and real kubelet restart are different operations: the automated Unix-socket harness simulates kubelet socket recreation; Day 7 live restart uses the DaemonSet, not a kubelet service restart.

Inspect state and logs:

```sh
kubectl --context gpu-node-selector -n ai-system get pods -o wide
kubectl --context gpu-node-selector -n ai-system logs daemonset/fake-gpu-device-plugin
kubectl --context gpu-node-selector get nodes -o custom-columns='NAME:.metadata.name,GPU-CAPACITY:.status.capacity.ai\.dhiraj\.io/gpu,GPU-ALLOCATABLE:.status.allocatable.ai\.dhiraj\.io/gpu'
```

The proof cleans up its temporary workload automatically. The DaemonSet remains installed for Day 8. Stop the project without destroying it using `make cluster-down`; use `make cluster-up` to resume later. Do not use `cluster-destroy` for routine shutdown.

## Automated verification

```sh
make fmt vet test build
SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk make race
make generate manifests envtest check-generated
bash -n scripts/plugin.sh
git diff --check
```

The SDK override addresses the existing host linker mismatch. Unit/RPC tests cover invalid counts, inventory, multi-device/multi-container allocation, unknown/duplicate IDs, repeat RPCs, canceled streams, readiness-before-registration, duplicate processes, socket recreation, shutdown and unrelated-file preservation. No real GPU is required.
