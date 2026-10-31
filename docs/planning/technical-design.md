# Technical implementation contract

Proposed decisions to implement after plan approval. Exact dependencies and command behavior must be verified during implementation; nothing here is test evidence.

## Final repository shape

```text
GpuNodeSelector/
├── PROJECT
├── api/v1alpha1/                 # AIWorkload, status, generated deepcopy
├── cmd/
│   ├── main.go                  # retain Kubebuilder manager convention
│   ├── fake-device-plugin/
│   └── mock-inference/
├── internal/
│   ├── controller/              # reconcile, mutation, status, pod mapping
│   ├── workload/                # pure builders and constants
│   ├── fakegpu/                 # inventory, gRPC, lifecycle
│   └── mockinference/           # HTTP handlers
├── config/
│   ├── crd/                     # generated schema
│   ├── default/
│   ├── manager/
│   ├── rbac/
│   ├── samples/
│   └── fake-device-plugin/
├── deploy/docker/Dockerfile     # three final targets
├── scripts/                     # guarded tools, cluster, images, demo, e2e
├── hack/                        # temporary manual capacity proof
├── tests/e2e/
├── docs/
│   ├── planning/                # this plan and daily context
│   ├── architecture.md
│   ├── reconciliation.md
│   ├── fake-device-plugin.md
│   ├── local-cluster.md
│   ├── status-and-failures.md
│   ├── demo.md
│   ├── troubleshooting.md
│   └── adr/
│       ├── 0001-default-scheduler.md
│       ├── 0002-fake-device-plugin.md
│       ├── 0003-gpu-node-isolation.md
│       └── 0004-owned-fields-and-rollouts.md
├── work/                        # ignored evidence
├── bin/                         # ignored binaries/local tools
├── .gitignore
├── Brewfile
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

Do not force `cmd/manager` if the selected scaffold uses `cmd/main.go`. Preserve existing planning files when scaffolding.

## Dependency choices and version gate

- Go for manager, plugin, and mock; standard net/http for mock; no AI SDK or external service.
- Kubebuilder's Go plugin, controller-runtime and its aligned k8s.io/api, apimachinery, client-go dependencies. Use the kubelet module's deviceplugin API matching the chosen Kubernetes minor and inspect supported protocol versions (commonly v1beta1); do not assume module stability across mismatched minors.
- gRPC for device plugin; a small context-aware socket lifecycle watcher, with fsnotify only if justified by the implementation.
- Kubebuilder-generated Kustomize deployment; controller-gen and setup-envtest pinned locally in the Makefile; no Helm dependency.
- Standard table-driven Go tests; keep generated Ginkgo/Gomega envtest suite if conventional for the selected scaffold. Envtest exercises admission and controller API interactions, not scheduling or GC.
- Colima + Docker/Buildx + Minikube Docker driver/containerd; jq for inspection. Native ARM64 first; AMD64 image build support with honest runtime-validation limits.
- Non-root minimal runtime images for manager/mock; plugin UID determined by kubelet socket-directory access. Do not set blanket privileged mode without a demonstrated need. Pin builder/runtime base versions or digests after validating availability.

Day 1 must record exact Go, Kubebuilder, controller-runtime, Kubernetes libraries, Kubernetes server, kubectl, controller-gen, Kustomize, envtest assets, Colima, Docker and Minikube versions. Use the scaffold's dependency compatibility set and verify server/CLI skew. A Brewfile alone does not pin versions. Commit Go sums and generator versions; record host tool versions separately. Do not fill this matrix with the reference project's historical versions.

## API contract

Namespaced `ai.dhiraj.io/v1alpha1`, kind `AIWorkload`, plural `aiworkloads`, status subresource.

| Spec field | Go representation | Admission / semantics |
| --- | --- | --- |
| image | string | required, nonempty image reference; registry existence not checked at admission |
| replicas | *int32 | default 1, minimum 0; pointer preserves explicit zero |
| port | *int32 | default 8080, range 1–65535 |
| gpu | required struct | no implicit absent-GPU mode |
| gpu.product | string | required, nonempty valid Kubernetes label value, bounded to 63 characters |
| gpu.count | int32 | required integer >=1; no cap at cluster capacity |
| resources | optional struct | absence means omit CPU/memory requests |
| resources.cpu | *resource.Quantity | optional nonnegative decimal cores (up to 3 fractional digits) or integer millicores; exponent/binary/fractional-milli forms rejected in v1alpha1 |
| resources.memory | *resource.Quantity | optional valid nonnegative quantity |

Use supported CRD schema markers/CEL for quantity validation and prove server admission behavior with envtest. Do not merely rely on fake-client Go unmarshalling. If installed CRD validation cannot enforce a desired semantic constraint, record and resolve it before marking Day 3 complete. CPU/memory translate to requests only; no invented CPU/memory limits. GPU count translates to integer limit and equal request (Kubernetes can default the request from the limit, but explicit equality simplifies inspection).

Canonical sample name `mock-llama`; valid short DNS-1035 child names match the CR. Day 4 derives a bounded prefixed/hash name for other CR names and stores the full CR name in an annotation. Selectors use a bounded UID hash; identity stays immutable throughout the CR's lifetime. Future status DNS must use workload.ChildName rather than blindly copying a long CR name.

Pod template: product selector `ai.dhiraj.io/gpu-product=A100`, GPU-node label selector `ai.dhiraj.io/gpu-node=true`, Equal toleration key `ai.dhiraj.io/gpu`, value `true`, effect `NoSchedule`. No nodeName, manual binding, capacity-based replica capping or node writes. The scheduler chooses the node. Labels describe eligibility; extended-resource requests account for capacity; taints exclude ordinary pods without tolerations.

Service type ClusterIP, port from spec, named target port `http`. Preserve clusterIP, clusterIPs, IP families and server defaults; update only managed selector/ports/type and owned metadata. Endpoint is `http://<name>.<namespace>.svc:<port>` once the Service exists; endpoint presence does not imply readiness.

Status uses observedGeneration, phase, desired/updated/ready/available/pendingReplicas, endpoint and []metav1.Condition with condition observedGeneration/lastTransitionTime. Counters are observations, not a scheduler capacity calculator. During rollout, ready counts can reflect old revision pods; Running requires updated and available desired replicas plus Deployment convergence. Exclude terminating pods from pending count and document whether pending covers all active owned Pods in phase Pending (proposed: yes). Desired/updated/etc. need not sum to one another during a rollout.

Zero desired replicas means a converged Running state with Ready=True/ScaledToZero, Scheduled=True/NoReplicasRequested, CapacityAvailable=Unknown/NoDemand; this is not proof of spare capacity. Use conditions as the machine contract; phase is a human-friendly summary.

## Owned fields and reconciliation

CreateOrPatch is proposed because focused mutation functions can retain unrelated fields explicitly. This is a deliberate merge policy, not full-object replacement. Owned fields include controller owner reference, identity labels, replicas, selector, rollout strategy, main container image/ports/probes/resources/security, required selectors/tolerations and Service selectors/ports/type. Preserve unrelated annotations/labels and safely merge unrelated tolerations/selectors. Reject contradictory configuration rather than silently claiming readiness. Specify behavior for extra containers in documentation; do not accidentally erase them by replacing the whole pod template.

A same-name child without this CR's controller UID is a collision, even if unowned. Do not adopt it automatically. Preserve non-controller owner references where safe. Child labels and Pod mapping are not sufficient authorization to adopt resources. Validate current Deployment/ReplicaSet owner chain to exclude forged/stale labels.

Pseudocode:

```text
Reconcile(key):
  fetch AIWorkload
  if NotFound: return success
  if other fetch error: return error
  if deletionTimestamp set: return success (normal GC, no finalizer)
  defensively validate desired spec
  if invalid: patch meaningful Failed status if changed; return bounded outcome

  build desired Deployment and Service with stable identity
  for each child:
    get/create with CreateOrPatch
    if existing and controller UID != AIWorkload UID:
      record OwnershipConflict status; return contextual error
    merge only operator-owned fields; preserve server allocations/defaults
    if API error: return error (controller-runtime retries/backoff)

  read observed Deployment and relevant active Pods
  derive counters and conditions from observed child generations/state
  set observedGeneration to CR generation processed
    (Ready remains false if child controller observations are stale)
  compare semantic status, preserving stable condition transition times
  patch status with conflict protection only if changed
  return success; bounded refresh only for unresolved asynchronous state
```

Watch CRs (generation/deletion semantics), owned Deployments/Services, and indirectly owned Pods through an identity-aware mapping. Do not apply a generation-only predicate globally: that would drop child status and Pod scheduling transitions. API conflicts should return for normal retry. No reconciliation sleeps, manual allocation locks, or global mutable state.

OwnerReferences give real-cluster garbage collection; no external cleanup means no finalizer. RBAC: CR read/watch plus status patch/update, children read/watch/create/update/patch as required, Pods/ReplicaSets read/watch for status and mapping, events only if emitting/reading them, leader-election lease permissions if enabled. No nodes/status writes or pods/binding permission in the operator. Bootstrap alone may label/taint/diagnostically patch nodes.

For two fully allocated GPUs, use RollingUpdate maxSurge=0, maxUnavailable=1. One old pod terminates to free a GPU before a replacement can start. This sacrifices some availability, especially at replicas=1; default surge-only rollout may otherwise stall indefinitely.

## Fake device plugin protocol

```text
start process; validate device count and socket path
  → inventory fake-gpu-0 .. fake-gpu-(N-1), all Healthy
  → serve device plugin gRPC on own socket under /var/lib/kubelet/device-plugins
  → register own endpoint/resource/API version at kubelet.sock
kubelet → GetDevicePluginOptions (supported options only)
kubelet → ListAndWatch stream → inventory → Node capacity and allocatable
scheduler → assigns eligible Pod based on remaining allocatable requests
kubelet → Allocate(container device IDs)
plugin → validate IDs → Envs[FAKE_GPU_DEVICES] = stable comma-separated IDs
kubelet → container runtime → starts mock container
pod termination → kubelet releases allocation for reuse
socket removal/recreation → restart own endpoint if needed → re-register
SIGTERM/context cancel → close streams/server and own socket only
```

Implement required interface methods; unsupported optional behaviors should advertise false options and return protocol-appropriate responses. Do not delete kubelet.sock or another plugin's socket. Use bounded retries/backoff with cancellation. Plugin shutdown/restart should not claim that existing container allocations disappear immediately.

Duplicate rejection applies to malformed IDs repeated within one Allocate request (including repeated assignment across its container requests). A later identical Allocate RPC can be retried successfully. Device plugin API does not supply a general Deallocate callback; a permanent local 'allocated' map would cause leaks and reject valid retries. Kubelet manages lifetime and exclusivity. Device IDs are unique per node, not cluster-wide. No device files, CUDA, model weights, or real GPU compute are provided.

Manual capacity proof is an exclusive diagnostic mode: plugin absent, GPU consumers removed before teardown, resource key removed from Node status, allocatable checked, then plugin can be enabled. Capacity and allocatable remain 2 while devices are occupied; free capacity is inferred by the scheduler from Pod requests, not by expecting allocatable to decrement to zero.

## Status precedence and transitions

Evaluate all conditions; phase uses the first matching row. GPU-capacity classification must have supporting scheduler evidence and no stronger/mixed blocker.

| Observation | Phase | Ready reason | Other condition behavior |
| --- | --- | --- | --- |
| Invalid desired state or ownership conflict | Failed | InvalidSpec / OwnershipConflict | Progressing=False; CapacityAvailable=Unknown |
| Deployment reports progress deadline exceeded | Failed | ProgressDeadlineExceeded | Preserve underlying diagnostic evidence |
| Observed image pull failure/crash | Degraded | ImagePullFailure / ContainerFailure | CapacityAvailable unknown unless independently proven |
| Current unscheduled pods: GPU shortage is effective blocker | CapacityBlocked | InsufficientGPU | Scheduled=False; CapacityAvailable=False |
| Product mismatch, untolerated taint, other/mixed constraints | Pending | GPUProductMismatch / SchedulingBlocked / MultipleSchedulingConstraints | Scheduled=False; CapacityAvailable=Unknown unless evidence supports False |
| Children missing/new; deployment observation stale | Provisioning | Reconciling / AwaitingObservation | Progressing=True; Scheduled/Capacity unknown as appropriate |
| Rollout advancing without diagnosed blocker | Provisioning | RollingOut | Progressing=True |
| Previously ready workload loses availability | Degraded | DeploymentUnavailable | Distinguish temporary progression from stalled availability |
| Desired=0, children converged | Running | ScaledToZero | Ready=True; CapacityAvailable=Unknown/NoDemand |
| Current deployment revision converged, desired replicas available | Running | Available | Ready=True; Scheduled=True; Progressing=False; CapacityAvailable=True/RequestsSatisfied |

Use PodScheduled=False/Unschedulable and container waiting reasons first. Scheduler messages are needed to distinguish extended GPU shortage from selector/taint/CPU-memory constraints. Preserve combined evidence, isolate parsing, and test unknown messages. `CapacityAvailable=True` means the current requested workload is satisfied; it does not advertise unused GPUs. Do not classify an unsatisfiable request for 3 GPUs on a 2-GPU node as a controller error: scheduler leaves it Pending and status explains it.

Day 6 implementation refinement: stale desired/Deployment observations gate all child diagnosis before the table is evaluated. Generic selector failures report NodeSelectorMismatch, not GPUProductMismatch, because the scheduler does not identify the specific label. Taints report UntoleratedTaint. Running additionally requires current owned Pods to be scheduled and ready, with no active old revision or terminating Pods. Mixed GPU and other scheduler clauses remain Pending/MultipleSchedulingConstraints. Full implemented semantics and evidence limitations are in docs/status-and-failures.md.

## Test matrix and evidence

| Coverage | Unit | envtest | Real cluster |
| --- | --- | --- | --- |
| Defaults/validation and generated schema | type/helper tests | API admission authoritative | install/sample smoke |
| Builders, selectors, resources, probes, zero replicas | table-driven | API default interaction | placement/probes |
| Create/repeat/update, no duplicates, owner collision | mutation helpers | watch/reconcile and persisted state | runtime smoke |
| Preserve clusterIP/unrelated metadata; drift repair | merge behavior | API assertions and stable writes | manually patch managed fields |
| Status phases, mixed failures, transitions, generation | table-driven | patch counts and event mapping | actual Pending/Running/failure observations |
| Inventory, Allocate validation, retries | gRPC/local socket/race | not applicable | IDs injected into running containers |
| Registration/socket restart | fake kubelet harness | not applicable | plugin restart; targeted kubelet recovery |
| Three nodes, CPU worker, GPU-only resource=2, label/taint | not applicable | not applicable | original e2e cases 1–5 |
| Two replicas Running, GPU placement, device IDs | not applicable | not applicable | original cases 6–8; IDs distinct concurrently |
| Three replicas Pending + InsufficientGPU status | classifier | simulated Pod states | original cases 9–10 |
| Scale down, second CR consumes released GPU | not applicable | replica spec only | original cases 11–12 |
| CR deletion, children/pods removed, capacity reused | owner references | no GC assertion | original cases 13–14 |
| Drift, unchanged CR, successful mock requests | builders/HTTP | reconciliation | original cases 15–17 |
| Image/GPU/product/port changes, saturated rollout | desired state | template updates | rollout and service endpoint assertions |
| Zero replicas, controller restart, child recreation | unit where useful | child recreation | recovery/concurrent CRs |

Save command logs, versions, CR JSON, deployments, Pods, nodes, events and relevant logs under `work/day-NN/` or timestamped `work/e2e/`. Avoid kubeconfig/credentials. Record actual commands/exit codes in handoff docs. `make verify` should compose format check, vet, unit/race, generation reproducibility, envtest and e2e with explicit prerequisites; it must fail or clearly report a missing prerequisite, never silently skip and call the whole suite passed.

## Original proposed demo command contract

Day 10 implementation refinement: the individual demo-* helpers below are preserved as the original design, not runnable commands. Their acceptance assertions are implemented by the shared Day 8/9 runner, exposed as `make demo` (full baseline), `make demo-smoke` (integration subset), `make verify` (all checks plus baseline) and `make e2e-recovery` (explicit restarts). Current commands are in README.md and docs/demo.md. Mixed scheduler evidence retains the Day 6/8 classification refinement above.

These targets/scripts will be implemented by Days 8–10. They do not exist yet. Default profile/namespace values are assumed below. Demo helpers use UID-aware selectors, bounded polling, unique run IDs, and diagnostic collection; they must refuse unrelated/preexisting fixture ownership.

```sh
cd /Users/dhirajsriharsha/Preparation/GpuNodeSelector
make doctor
make cluster-up
make cluster-configure
make docker-build
make minikube-load
make install
make deploy
make plugin-verify
kubectl --context gpu-node-selector get nodes -o wide
kubectl --context gpu-node-selector get nodes -l ai.dhiraj.io/gpu-node=true -o yaml
kubectl --context gpu-node-selector apply -f config/samples/ai_v1alpha1_aiworkload.yaml
make demo-wait PHASE=Running DESIRED=2 READY=2 PENDING=0
kubectl --context gpu-node-selector -n ai-workloads get aiworkloads,pods,deployments,services -o wide
make demo-isolation
make demo-http
kubectl --context gpu-node-selector -n ai-workloads patch aiworkload mock-llama --type=merge -p '{"spec":{"replicas":3}}'
make demo-wait PHASE=CapacityBlocked DESIRED=3 READY=2 PENDING=1
kubectl --context gpu-node-selector -n ai-workloads get events --field-selector reason=FailedScheduling
kubectl --context gpu-node-selector -n ai-workloads patch aiworkload mock-llama --type=merge -p '{"spec":{"replicas":1}}'
make demo-wait PHASE=Running DESIRED=1 READY=1 PENDING=0
make demo-capacity-reuse
make demo-drift
make demo-rollout
kubectl --context gpu-node-selector apply -f config/samples/ai_v1alpha1_aiworkload.yaml
make demo-wait PHASE=Running DESIRED=2 READY=2 PENDING=0
make demo-stability
kubectl --context gpu-node-selector -n ai-workloads delete aiworkload mock-llama --wait=true
make demo-deletion-verify
make demo-reuse-after-delete
make verify
make cluster-down
```

Helper contracts:

- `deploy`: validate no manual diagnostic mode; apply plugin/manager and namespaces, wait for availability. `plugin-verify`: GPU worker capacity=allocatable=2, other nodes have none, DaemonSet on GPU worker only.
- `demo-wait`: verify CR observedGeneration equals metadata.generation, requested phase/counters and Deployment convergence as applicable, not just stale Ready=True. Default timeout 180 seconds, overridable; emit diagnostics on failure.
- `demo-isolation`: CPU-selected ordinary pod runs; GPU-selected pod without toleration remains unscheduled; clean up both. This proves more than checking where one random ordinary pod landed.
- `demo-http`: temporary managed port-forward, health/readiness/device/inference requests; inspect each Pod for unique assigned IDs; release forwarding process on exit. Use unused configurable local port.
- `demo-capacity-reuse`: apply a second one-replica fixture after primary scaled to one, verify 2 total GPUs used, delete second fixture and wait for its Pods to vanish.
- `demo-drift`: mutate one managed field at a time, including replicas/GPU limit/selector/toleration; observe restoration and preserve unrelated annotation. Restore baseline and wait between cases.
- `demo-rollout`: fill both GPUs, build/load a distinct mock image tag, update CR, prove rollout completes with zero surge, return to sample image/two replicas. Distinct image tag is necessary even if response body is unchanged.
- `demo-stability`: reapply identical CR and compare owned spec/status/resource changes over an observation window after convergence; permit unrelated Kubernetes controller updates, forbid operator-induced churn.
- `demo-deletion-verify`: wait for Deployment, Service and all previously captured owned Pod UIDs to be absent, not only for the CR deletion response.
- `demo-reuse-after-delete`: temporary two-replica second CR reaches Running, then delete it and wait for cleanup. Does not recreate the deleted primary implicitly.
- `make demo`: orchestrates the same flow through these helpers, excluding cluster shutdown so the user can inspect results.

## Primary technical references checked during planning

- [Kubernetes device plugin framework](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/): registration, allocation, socket lifecycle and device resources.
- [Kubernetes extended resource scheduling](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#extended-resources): capacity/allocatable and resource requests.
- [Kubebuilder envtest](https://book.kubebuilder.io/reference/envtest.html): API-only test environment; no built-in controllers/garbage collection.
- [Kubebuilder artifacts/tool versions](https://book.kubebuilder.io/reference/artifacts.html): generated tool/version alignment.

Recheck installed-version documentation on Day 1 and API details on Day 7; current documentation is guidance, not evidence of the uninstalled local versions.
