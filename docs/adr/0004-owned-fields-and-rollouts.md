# ADR 0004: Merge owned fields and use zero-surge workload rollouts

Status: Accepted implementation, recorded 29 October 2026.

Context: The operator repeatedly reconciles API objects that also have Kubernetes defaults and user metadata. Replacing whole objects can erase Service allocations or cause perpetual diffs. Both fake devices may already be occupied during an image update.

Decision: Use focused CreateOrPatch mutations after verifying the child controller UID. Own the inference container, required scheduling/security fields, replica count, strategy and identity. Preserve unrelated metadata, other containers/volumes and unrelated selectors/tolerations. Preserve Service clusterIP/IP-family allocations. Reject foreign or unowned same-name children rather than adopt them. Compare semantic status and preserve condition transition times when the condition status is unchanged.

Use maxSurge=0/maxUnavailable=1 for workload Deployments. Terminating Pods may overlap replacement Pod objects, but a replacement cannot run until a device is released. Use one operator replica with Recreate because leader election is not enabled. OwnerReferences provide child cleanup; no external obligation justifies a finalizer.

Consequences: Single-replica updates may briefly interrupt inference, while an operator restart leaves existing inference Pods running. The entire inference container is managed, so admission injectors must not compete for its fields. Additional user scheduling constraints can prevent placement and are reflected in status. Real-cluster tests prove drift repair, stable reapply, Service preservation, deletion/reuse and saturated rollout; envtest alone cannot prove scheduling or garbage collection.
