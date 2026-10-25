# Workload status and failure diagnosis

Day 6 derives AIWorkload status from its owned Deployment and Pods. The operator observes scheduling; it never binds Pods, reads free GPU capacity, mutates Nodes or reduces requested replicas.

## Observations and readiness

Pods must match the current CR's UID-derived identity and name annotation, and have a current Pod → ReplicaSet → Deployment owner chain. ReplicaSet templates are compared with the Deployment template, ignoring only pod-template-hash. This distinguishes current and old rollout revisions without relying on an image tag alone.

`observedGeneration` records the CR generation processed, including failures. Every condition carries that generation. Running additionally requires the child spec to match current desired fields and Deployment status.observedGeneration to reach its generation. A pre-patch cache read or stale Deployment observation yields Provisioning/AwaitingObservation, even if old counters look ready.

| Field | Meaning |
| --- | --- |
| desiredReplicas | Exact CR replica request, including zero |
| updatedReplicas | Deployment's observed updated replica count |
| readyReplicas | Deployment's observed ready count, potentially including old revision Pods |
| availableReplicas | Deployment's observed available count |
| pendingReplicas | All nonterminating owned Pods in phase Pending, including old revisions and already-scheduled Pods waiting to start |
| endpoint | Owned Service DNS and desired port; presence does not prove readiness |

Counters need not sum to one another. Stale child counters remain visible while conditions explain AwaitingObservation. Terminating Pods are excluded from Pending and failure classification. Old revision failures do not classify the current rollout; active old revision Pods still prevent convergence. Historical terminal Pods do not mask healthy replacements. A failed current-revision Pod without an active replacement reports ContainerFailure.

Running requires matching desired/updated/ready/available Deployment counts, zero unavailable replicas, exactly the requested active current-revision Pods with PodReady and PodScheduled true, no active old revision Pods and no terminating owned Pods. At zero replicas, all these counts must reach zero; CapacityAvailable remains Unknown/NoDemand.

## Conditions and phase precedence

All four conditions are always present: Ready, Progressing, Scheduled, CapacityAvailable. Unknown means evidence is absent or insufficient, not False.

| Evidence, in priority order | Phase / Ready reason |
| --- | --- |
| Invalid desired input or ownership conflict | Failed / InvalidSpec or OwnershipConflict |
| Child configuration missing/stale or Deployment observation stale | Provisioning / AwaitingObservation |
| Current Deployment Progressing=False, ProgressDeadlineExceeded | Failed / ProgressDeadlineExceeded |
| Current Pod image-pull, crash, config or container failure | Degraded / ImagePullFailure or ContainerFailure |
| Only GPU shortage reported for blocked current Pods | CapacityBlocked / InsufficientGPU |
| Multiple scheduling cause categories | Pending / MultipleSchedulingConstraints |
| Selector mismatch, taint, or unrecognized blocker | Pending / NodeSelectorMismatch, UntoleratedTaint or SchedulingBlocked |
| Current revision converged with zero demand | Running / ScaledToZero |
| Current revision converged with positive demand | Running / Available |
| Previously available same-generation revision loses readiness/availability | Degraded / DeploymentUnavailable |
| Remaining rollout or startup progress | Provisioning / RollingOut |

The stale-observation gate precedes child diagnosis so old generation failures cannot describe a newer desired revision. Progressing=True describes startup/rollout awaiting observations; blocked, failed, degraded and converged states use False. Scheduled reflects current Pod scheduling evidence independently of image health. CapacityAvailable=True/RequestsSatisfied means this workload's demand is satisfied, never that unused GPUs exist. Independently proven capacity shortage may remain False alongside a higher-priority container/deadline failure.

## Conservative scheduler parsing

Structured PodScheduled=False and reason Unschedulable are required before parsing messages. The parser accepts at most 4096 bytes (larger messages become unknown), ignores the separate preemption explanation, and recognizes complete known clauses for `Insufficient ai.dhiraj.io/gpu`, node affinity/selector mismatch and untolerated taints. Unknown clauses remain blockers. GPU evidence combined with another category, within one message or across Pods, is not capacity-only.

Generic selector text cannot prove which label is wrong. It reports NodeSelectorMismatch rather than claiming GPUProductMismatch: an intentional conservative refinement of the planning examples. The three-node cluster can report CPU/control-plane exclusions alongside GPU shortage; that yields MultipleSchedulingConstraints. Real scheduler evidence and GPU integration are verified on Day 8.

Container waiting reasons include ErrImagePull, ImagePullBackOff, InvalidImageName, CrashLoopBackOff, CreateContainerConfigError, CreateContainerError and RunContainerError. Init containers and sidecars are included. Nonzero current termination exit codes indicate failure; historical LastTerminationState alone does not mark a recovered container failed.

Messages are stable summaries, without timestamps, changing scheduler node counts or unordered Pod lists. Inspect original Pod conditions/events for detailed evidence.

## Idempotency and retries

Only a semantic status change produces a resourceVersion-protected patch. LastTransitionTime changes only when the final condition Status changes; reason/message/generation updates preserve it. Comparing final conditions prevents internal Unknown-to-final calculations from creating artificial transitions.

Existing CR, child and Pod watches drive updates. Unresolved nonterminal states request a 15-second refresh for cache/owner-chain gaps; Running and Failed have no periodic refresh. API errors return for controller-runtime backoff. No sleeps occur inside reconciliation, and Node RBAC is unchanged.

## Verify Day 6

```sh
cd /Users/dhirajsriharsha/Preparation/GpuNodeSelector
make generate manifests fmt vet test build envtest check-generated
SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk make race
git diff --check
```

The SDK override addresses this host's default SDK/linker incompatibility. Tests use temporary envtest API servers, not the project cluster. Unit tables exercise zero replicas, rollouts, stale generations, failure precedence, mixed/unknown scheduler messages, old/terminating Pods, counters and timestamps. Envtest proves persisted status idempotency and a running manager's response to Pod readiness events. These simulate observations; they do not prove actual scheduling or device allocation.

For optional live inspection, use the Day 5 guide to start the cluster, install the CRD, run `make run` and create its zero-replica fixture. With the real Deployment controller it should reach Running/ScaledToZero:

```sh
kubectl --context gpu-node-selector -n ai-day5-check wait --for=condition=Ready aiworkload/day5-check --timeout=60s
kubectl --context gpu-node-selector -n ai-day5-check get aiworkload day5-check -o yaml
```

Printed columns remain Phase, Desired (spec), Ready and Pending (status). A fixture with one unscheduled Pod prints Pending=1; converged zero replicas print Running, Desired=0, Ready=0, Pending=0. Use YAML to compare metadata.generation, status.observedGeneration and reasons; numeric columns alone do not establish current readiness. Follow Day 5 cleanup afterward. Live checks were not run during Day 6 implementation.
