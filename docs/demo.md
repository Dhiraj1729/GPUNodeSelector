# Demonstration and verification

The final `make demo` runs the complete baseline acceptance walkthrough, including the Day 8 integration flow below and the Day 9 drift/stability, isolation, garbage collection/reuse, collision and failure scenarios. It cleans its generated namespaces and leaves the platform installed. `make demo-smoke` remains the shorter Day 8 subset. Both require no competing GPU consumers. `make verify` combines format/vet/unit/build/generation/envtest/race/shell checks, image/security/RBAC audit and the complete baseline. It fails if prerequisites are missing; it never silently skips cluster tests. Recovery restarts remain the separate explicit `make e2e-recovery` target.

Final demo evidence is under work/day-10/demo; verification evidence is under work/day-10/verify. Detailed acceptance mapping is in tests/e2e/README.md. The planned individual demo-* helpers were consolidated into the tested E2E runner to avoid maintaining duplicate assertions; the public workflow is make demo, make demo-smoke and make e2e-recovery.

The operator runs in ai-system using its ServiceAccount and generated RBAC. It creates Deployment/Service children; the default scheduler selects the GPU worker; kubelet allocates plugin devices and injects FAKE_GPU_DEVICES; the CPU-only mock reports that identity through HTTP. No manual Node capacity patch is used.

## Run from a fresh or preserved project cluster

```sh
cd /Users/dhirajsriharsha/Preparation/GpuNodeSelector
make tools doctor                 # tools installs missing tools only
make cluster-up
make cluster-configure
make docker-build minikube-load install deploy
make demo-smoke
make plugin-verify
```

For final acceptance use `make verify` and `make demo`. Stop with `make cluster-down`; restart with `make cluster-up`, `make deploy`, `make demo`. For permanent removal use `make cluster-destroy CONFIRM=gpu-node-selector`, which deletes Minikube, the Colima VM and its runtime data disk. Source and evidence remain. See docs/local-cluster.md for lifecycle guarantees and the Day 10 handoff for actual execution evidence.

For a fresh cluster, plugin-deploy requires GPU capacity to be absent before the plugin registers. For an existing Day 7 cluster, it verifies the project-owned DaemonSet and reuses it. Do not run the Day 2 manual-capacity proof with the plugin deployed. If a prior manual diagnostic resource blocks first deployment, follow the guarded removal procedure in docs/fake-device-plugin.md.

`deploy` is safe to rerun: one operator Deployment (Recreate, one replica), one plugin DaemonSet, one operator ServiceAccount, generated ClusterRole and binding. It rejects existing platform resources without the project ownership label before applying anything. CRD installation accepts its project label or migrates the exact Day 3 last-applied schema once; it rejects unrelated/unrecognized definitions. Existing ai-system namespace metadata is preserved. The operator authenticates in-cluster and has no Node/binding permission. Do not run a second local `make run` manager concurrently; leader election is intentionally not enabled for this single-instance milestone.

The operator is a static non-root scratch image with a read-only root filesystem, no privilege escalation, all capabilities dropped, RuntimeDefault seccomp, CPU/memory requests, and health/readiness probes on 8081. Recreate prevents overlapping managers during image updates; managed inference Pods continue running while the manager restarts.

## What demo-smoke proves

The script creates a server-generated `ai-workloads-day8-*` namespace. This is deliberate isolation instead of adopting a user's existing ai-workloads namespace. All test workloads are removed on exit, including failure; platform components remain installed.

1. Validates the three-node topology, exact image IDs in every node's containerd store, operator rollout, plugin placement/capacity and a focused allocation proof.
2. Creates `first` with two replicas, A100, one GPU per Pod, 100m CPU and 128Mi memory. Checks owned Deployment/Service, current Running status, GPU-node placement, requests/limits and distinct per-Pod device responses.
3. Scales through the CR to three. Checks two Running Pods, one Pending/Unschedulable Pod, raw GPU-shortage evidence, unchanged desired=3, and matching operator classification.
4. Scales to one, waits for convergence/draining, creates `second`, and proves released capacity is reused with distinct simultaneous IDs.
5. Calls healthz, readyz, device and POST inference through the Service API proxy and saves deterministic mock responses.
6. Changes first's port to 9090. Checks PORT, container/probe ports, Service port/target, endpoint, preserved clusterIP and unrelated user-note annotation, readiness and HTTP responses.
7. Builds a unique `day8-<run-id>` mock image tag with distinct OCI version metadata, loads and verifies it on all nodes, and updates first while second still occupies the other device.
8. Checks maxSurge=0/maxUnavailable=1, rollout completion, new Pod UID/tag/image ID, valid device response, unchanged Ready second Pod, and no additional GPU capacity. Saves a Pod timeline and events.

Scheduler truth matters: on the verified cluster, the message was `1 Insufficient ai.dhiraj.io/gpu, 2 node(s) didn't match Pod's node affinity/selector`. Correct status is Pending/MultipleSchedulingConstraints with CapacityAvailable=Unknown. The test independently checks whether every pre-preemption scheduler clause is GPU shortage; only that capacity-only case requires CapacityBlocked/InsufficientGPU. It does not weaken diagnosis to force a demo label.

Zero surge does not prohibit a replacement Pod object while the old Pod is terminating. The verified timeline showed the old Pod draining and the replacement briefly Pending, then Running after capacity release. The test bounds nonterminating first Pods to one, verifies the second Pod remains Ready, and requires eventual convergence. It does not falsely claim creation must wait for complete old-Pod deletion.

## Images and cache safety

`scripts/images.sh` builds operator, fake-device-plugin and mock-inference for the cluster's native architecture by default. Build and load are separate: Colima Docker images are not Minikube containerd images. Load uses the named Minikube profile, then inspects every node using crictl and compares both repo tags and config digests against Docker. Missing/stale or architecture-mismatched images fail verification. Workloads use IfNotPresent locally.

The operator Deployment has a Docker image-ID annotation, so a changed dev build triggers replacement while an identical redeploy does not. For inference rollouts, always use a new tag and load it before changing the CR. To rebuild the plugin under dev, load the image then use `make plugin-restart`.

```sh
COMPONENTS=mock-inference IMAGE_TAG=my-v2 make docker-build minikube-load
PLATFORM=linux/amd64 IMAGE_TAG=day8-amd64 make docker-build
```

Cross-builds must use a separate tag; do not load AMD64 images into ARM64 Minikube. All three AMD64 cross-builds passed; AMD64 runtime was not validated.

## Evidence and tests

Each smoke run writes work/day-08/<UTC-run-id>/ with stage snapshots, raw scheduling conditions/events, per-Pod device responses, endpoint JSON, image builds/load logs, per-node inventory, revisions/ReplicaSets, Pod timeline and platform logs. Top-level files retain platform deployment, regression results, cross-build output and before/after operator UID evidence. Errors trigger diagnostics and cleanup. Cleanup failure returns nonzero and prints the generated namespace for inspection; do not delete unrelated namespaces.

```sh
SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk \
  make generate manifests fmt vet test build envtest check-generated race
bash scripts/test-platform-ownership.sh
bash -n scripts/images.sh scripts/deploy.sh scripts/demo.sh scripts/install-crd.sh
git diff --check
```

The SDK workaround is specific to the existing host linker mismatch. Integration proof uses real Kubernetes scheduling/kubelet/device-plugin allocation, but inference remains a deterministic mock, not physical GPU computation.

## Optional persistent sample and shutdown

The smoke leaves no inference workloads. For manual inspection, create ai-workloads if it does not exist, then apply config/samples/ai_v1alpha1_aiworkload.yaml. Remove that sample before rerunning smoke, because it consumes the two devices.

```sh
kubectl --context gpu-node-selector create namespace ai-workloads
kubectl --context gpu-node-selector apply -f config/samples/ai_v1alpha1_aiworkload.yaml
kubectl --context gpu-node-selector -n ai-workloads wait --for=condition=Ready aiworkload/mock-llama --timeout=180s
kubectl --context gpu-node-selector -n ai-workloads get aiworkloads,pods,deployments,services
kubectl --context gpu-node-selector -n ai-workloads delete aiworkload mock-llama
make cluster-down
```

Namespace creation deliberately fails if it already exists; inspect/reuse your existing namespace rather than deleting it. Shutdown preserves platform deployment/images. Resume with cluster-up and rerun verification. Destruction remains a separate explicit operation.
