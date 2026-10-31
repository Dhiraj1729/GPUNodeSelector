# Desired-state construction and reconciliation

Day 4 implements `workload.Deployment` and `workload.Service` in internal/workload. These pure functions take a persisted AIWorkload and return fresh desired Kubernetes objects or an error. Day 5 connects them to API mutation and watches through internal/controller. Neither layer reserves GPU capacity or caps replicas.

## Translation contract

| Input | Desired state |
| --- | --- |
| image | Inference container image, local IfNotPresent pull policy |
| replicas | Exact requested replicas; absent defaults to 1 and explicit zero is preserved |
| port | Container port, PORT environment variable and Service port; absent defaults to 8080 |
| gpu.count | Equal integer ai.dhiraj.io/gpu request and limit, with no capacity cap |
| gpu.product | Required ai.dhiraj.io/gpu-product node selector |
| GPU workload | ai.dhiraj.io/gpu-node=true selector and Equal ai.dhiraj.io/gpu=true:NoSchedule toleration |
| CPU/memory | Optional requests, deep-copied; no CPU/memory limits invented |
| CR identity | Controller OwnerReference containing CR name, UID, kind and API version |

Pod nodeName is empty and no custom scheduler is selected. Device environment variables come from the plugin's Allocate response through kubelet. Probes target named port `http` at `/healthz` and `/readyz`. The container is non-root UID/GID 65532, with all capabilities dropped, no privilege escalation, a read-only root filesystem and RuntimeDefault seccomp. Service-account token mounting is disabled. Graceful Pod termination allows ten seconds, exceeding the mock server's five-second drain deadline.

RollingUpdate uses maxSurge=0 and maxUnavailable=1. This permits replacement when both GPUs are occupied by removing an old replica first. Single-replica updates can interrupt service. Day 8 verified saturated rollout: the replacement can be Pending while the old Pod terminates, then runs after capacity release. See docs/demo.md.

## Stable names and selectors

Valid short DNS-1035 CR names, including mock-llama, are retained as child names. Names unsuitable for Services (long names, names containing dots, or names beginning with digits) become an `ai-` prefix, bounded readable segment and hash of the complete name. Both child types use the same derived name. This accommodates the Day 3 API's longer object names without introducing a breaking name restriction. Future status endpoint construction must use this derived Service name.

Selectors use a bounded SHA-256 digest of the CR UID in ai.dhiraj.io/workload-id. Full CR name is stored in ai.dhiraj.io/workload-name annotations rather than a potentially invalid label. Deployment selector, Pod labels and Service selector match. Spec updates never change selector identity; replacing the CR with a new UID produces a new identity. The Pod watch validates the full current ownership chain and UID, not the annotation alone.

Builders reject nil input, invalid name/namespace, missing persisted UID, missing image, invalid GPU count/product, negative replicas, invalid port, negative requests and fractional-millicore CPU requests. API defaulting remains authoritative; local pointer fallbacks exist for independently constructed objects. Quantity validation here is numeric because native Quantity parsing normalizes wire representations.

## Day 5 mutation boundary

The Service builder leaves clusterIP, clusterIPs and IP-family allocations unset. The reconciler merges only selector, ports and type into the existing Service, preserving its allocations and other defaults.

The reconciler explicitly implements create-or-patch rather than calling controllerutil.CreateOrPatch: patches include a resourceVersion precondition, preventing concurrent edits or replacement objects from being overwritten. API conflicts return to controller-runtime for backoff and a fresh reconciliation. Semantic equality skips writes, including status writes. Day 6 adds a 15-second refresh for unresolved nonterminal status; no sleep is used in reconciliation.

Existing children must have the CR's controller UID, name, kind and API version. Unowned, foreign and stale-UID children are rejected untouched. Non-controller owner references and unrelated object/template labels and annotations survive. A mismatched immutable Deployment selector returns an error; the operator does not delete/recreate it to bypass Kubernetes immutability.

Managed Deployment fields are replicas, rollout strategy, identity, pod security, service-account-token mounting, termination grace, default scheduler and the entire named inference container. Other containers, volumes, unrelated node selectors and tolerations are retained. Every toleration with the GPU taint key is replaced by the single required exact toleration. Additional scheduling constraints may make Pods unschedulable; Day 5 never claims readiness. Changes inside the inference container, including injected mounts/env, are overwritten; admission injectors must not compete for these owned fields. Kubernetes-defaulted container/probe values are included when merging to avoid perpetual diffs. Other Pod/Deployment defaults remain intact.

Owned Deployment/Service events enqueue their CR. Pod events map through the full current Pod → ReplicaSet → Deployment → AIWorkload chain and the CR UID-derived identity label. Forged annotations and stale owner UIDs are rejected. No global generation-only predicate discards child status events.

Day 6 now derives four conditions, replica observations and generation-aware phase from the Deployment and verified Pods; see [status and failures](status-and-failures.md). Invalid input and ownership conflicts report Failed. Unchanged final status is not patched. OwnerReferences supply real-cluster garbage collection; no finalizer is installed. The RBAC finalizers update permission is solely for blockOwnerDeletion admission. The generated role has no Node, binding, Secret or child-delete permissions. Day 8 binds it to the in-cluster operator ServiceAccount.

## Day 5 verification

Run from the repository root:

```sh
make generate manifests fmt vet test build envtest check-generated
make race
git diff --check
```

The envtest suite starts temporary local API servers and etcd; it needs no Minikube or Docker. It checks API-defaulted idempotency by comparing Deployment, Service and CR resourceVersions over three repeat reconciles, as well as creation, spec changes, zero replicas, drift, recreation, ownership conflicts, metadata/IP retention and actual manager child watches. Unit tests cover Pod mapping and conflict retry. Envtest cannot prove scheduling, kubelet allocation or garbage collection.

If the host's default macOS 27 SDK fails with `unknown architecture arm64e.x1`, use the already-installed compatible SDK for the race check:

```sh
SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk make race
```

### Optional live verification, in order

1. Prepare the existing project cluster and CRD:

```sh
make cluster-up cluster-configure install
make run
```

`make run` builds the manager, validates project profile/context identity, and runs it in the foreground against `gpu-node-selector`. Stop with Ctrl-C. It uses your kubeconfig permissions. Day 8 deploys the manager in-cluster with generated RBAC; use docs/demo.md for that flow. Do not run this local manager while the in-cluster manager is running.

2. In a second terminal, create an isolated verification namespace (create fails if it already exists), and a zero-replica CR. This exercises the operator without requiring the Day 7 plugin:

```sh
kubectl --context gpu-node-selector create namespace ai-day5-check
kubectl --context gpu-node-selector -n ai-day5-check apply -f - <<'YAML'
apiVersion: ai.dhiraj.io/v1alpha1
kind: AIWorkload
metadata:
  name: day5-check
spec:
  image: gpu-node-selector/mock-inference:dev
  replicas: 0
  gpu:
    product: A100
    count: 1
YAML
kubectl --context gpu-node-selector -n ai-day5-check wait --for=create deployment/day5-check --timeout=60s
kubectl --context gpu-node-selector -n ai-day5-check wait --for=create service/day5-check --timeout=60s
kubectl --context gpu-node-selector -n ai-day5-check get aiworkloads,deployments,services
```

3. Update image/port/GPU requirements and inspect the Deployment template and Service. Replicas stay zero:

```sh
kubectl --context gpu-node-selector -n ai-day5-check patch aiworkload day5-check --type=merge -p '{"spec":{"image":"gpu-node-selector/mock-inference:day5","port":9090,"gpu":{"product":"H100","count":2}}}'
kubectl --context gpu-node-selector -n ai-day5-check get deployment day5-check -o yaml
kubectl --context gpu-node-selector -n ai-day5-check get service day5-check -o yaml
```

Expect GPU requests=limits=2, H100 product selector, GPU-node selector, exact NoSchedule toleration, port/PORT=9090 and the new image. The Service IP stays the same. Allow a few seconds for asynchronous reconciliation before inspecting.

4. Add unrelated metadata and drift the managed Service selector, then watch it return to the Deployment identity:

```sh
kubectl --context gpu-node-selector -n ai-day5-check annotate service day5-check user-note=keep
kubectl --context gpu-node-selector -n ai-day5-check patch service day5-check --type=json -p '[{"op":"replace","path":"/spec/selector","value":{"wrong":"selector"}}]'
kubectl --context gpu-node-selector -n ai-day5-check get service day5-check -o yaml
kubectl --context gpu-node-selector -n ai-day5-check delete service day5-check
kubectl --context gpu-node-selector -n ai-day5-check wait --for=create service/day5-check --timeout=60s
```

Metadata should survive drift repair. Deletion creates a fresh Service, so its UID/IP and user annotation need not survive recreation. Automated tests cover destructive ownership/collision cases safely in envtest.

5. Inspect the stable specs several times after convergence. Automated envtest provides the strict resourceVersion assertion; live Deployment controllers may independently update status. Clean up while the manager remains running, then stop it:

```sh
kubectl --context gpu-node-selector -n ai-day5-check delete aiworkload day5-check
kubectl --context gpu-node-selector -n ai-day5-check wait --for=delete deployment/day5-check service/day5-check --timeout=60s
kubectl --context gpu-node-selector delete namespace ai-day5-check
```

Optional shutdown afterward: `make cluster-down`. The live sequence is provided for you to run; it was not executed during Day 5 implementation.

## Verified on Day 4

Table-driven tests cover defaults/zero replicas, GPU counts above capacity, selectors/tolerations, resources, ports/probes, non-root security, rollout policy, OwnerReferences, Service shape, invalid inputs, long names, stable identity across updates, recreated CR identity, deterministic output and independent mutable data. `make race` passed on both new packages. No Kubernetes client is used by either builder.
