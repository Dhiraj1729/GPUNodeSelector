# AIWorkload v1alpha1 API

Day 3 implements a namespaced `ai.dhiraj.io/v1alpha1` resource with plural `aiworkloads`. Kubernetes accepts, defaults and validates it. The manager registers the API but has no reconcilers; applying a CR does not create a Deployment, Service or Pod yet.

The Kubebuilder 4.16.0 go/v4 scaffold was inspected in a staging directory, then adapted into the existing repository without replacing its scripts or Makefile. `PROJECT` records the API and module identity. `config/default` currently includes only the CRD. The viewer role is an optional, unbound RBAC template; installing the CRD grants no additional workload permissions. Manager deployment and controller permissions are deferred until a controller exists.

## Spec

| Field | Contract |
| --- | --- |
| image | Required nonempty string; admission does not contact a registry |
| replicas | Optional int32, default 1, minimum 0; explicit zero remains zero |
| port | Optional int32, default 8080, inclusive range 1–65535 |
| gpu | Required object |
| gpu.product | Required nonempty Kubernetes label value, maximum 63 characters |
| gpu.count | Required positive int32; values above local GPU capacity are valid |
| resources | Optional object; empty object is valid |
| resources.cpu | Optional nonnegative request in decimal cores with at most three fractional digits or integer millicores; e.g. `1`, `"0.001"`, `100m` |
| resources.memory | Optional nonnegative Kubernetes quantity; e.g. `128Mi`, `1G`, or `0` |

CPU and memory are native `resource.Quantity` pointers. Their wire schema accepts integer or string quantities, not JSON floating-point numbers: quote fractional values. Defaulting belongs to Kubernetes admission, not Go constructors or a webhook.

CPU syntax is deliberately narrower than all `resource.Quantity` encodings to express exact millicore precision without floating-point CEL arithmetic. Exponent notation (`1e-3`), binary CPU suffixes, fractional millicores (`0.1m`), and more than three decimal places are rejected. Use `1m` or `"0.001"` for a millicore. This restriction is applied to raw API input and documented rather than relying on Go deserialization. Memory uses the native quantity pattern plus CEL parsing and comparison against zero. Both allow zero; neither introduces a limit or a capacity-based maximum.

The sample at `config/samples/ai_v1alpha1_aiworkload.yaml` uses namespace `ai-workloads`; create that namespace separately if applying it manually. Its image is a future local image and is not needed for Day 3 API acceptance.

CR names retain Kubernetes object-name validation. No 63-character name limit was added solely for future labels. Day 4 builders must use bounded identity labels (for example a UID) and must not blindly copy long CR names into label values.

## Status

Status is a separate subresource. Creation and ordinary spec updates cannot set status; status updates cannot change spec and do not increment metadata.generation.

Fields: `observedGeneration`, `phase`, `desiredReplicas`, `updatedReplicas`, `readyReplicas`, `availableReplicas`, `pendingReplicas`, `endpoint`, and `conditions`. Generation and replica counters are nonnegative. Allowed phases: Pending, Provisioning, Running, Degraded, CapacityBlocked, Failed.

Conditions are `metav1.Condition` objects with list-map semantics keyed by `type`. Constants define Ready, Progressing, Scheduled and CapacityAvailable; reasons and status derivation are added when their behavior is implemented. Conditions retain the conventional observedGeneration, reason, message and lastTransitionTime fields. Status is initially empty because no reconciler computes it yet. Printer columns show phase, requested replicas, ready and pending counts; requested replicas comes from spec, so it is visible before reconciliation.

## Reproduce the verification

```sh
make generate manifests
make fmt vet test build
make envtest
make check-generated
make cluster-up
make install
make api-smoke
kubectl --context gpu-node-selector explain aiworkload.spec
```

`make test` runs ordinary Go tests. `make envtest` downloads pinned Kubernetes 1.37.0 assets and starts a local API server and etcd with no external cluster dependency. It runs 35 admission subtests using raw/unstructured submissions and typed reads, including invalid quantities, fractional integer fields, zero replicas, defaults, above-capacity requests, and spec/status isolation. Missing assets cause a failure rather than a skipped success. Envtest does not prove scheduling, Deployment behavior or garbage collection.

`make install` validates the existing project's ownership/profile/context before applying the CRD. `make api-smoke` creates a uniquely named temporary namespace, verifies defaults and a real validation rejection, records `kubectl explain`, and deletes only that namespace on exit. The CRD remains installed. Failure to clean up is reported through the command exit status.

Builds default to `CGO_ENABLED=0`, appropriate for this pure-Go manager. The host's macOS SDK linker failed on a C-enabled build; no system SDK files were changed. Native race testing requires a functioning C toolchain and was not claimed as verified.

Evidence is under ignored `work/day-03/`. Generated CRD YAML and deepcopy code are source artifacts; `make check-generated` fails if running the pinned generators changes them, including before the files have been committed.

## Validation implementation

Schema markers define required fields, bounds, enums and product syntax. CEL handles resource semantics directly in the API server; no webhook is needed. The initial memory expression used a quantity method unavailable to this server and was rejected by both envtest and the live cluster. The final expression uses `compareTo(quantity('0'))` and passed both. See the [Kubernetes CEL quantity library](https://kubernetes.io/docs/reference/using-api/cel/#kubernetes-quantity-library) for the library concepts; the actual server tests establish availability for this pinned version.
