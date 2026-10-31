# Local integration troubleshooting

Start with the failing run's work/day-09/<run>/run.log and exit-code. Final snapshots and events are collected before test namespace cleanup; operator/plugin logs and node JSON are retained. Nested integration evidence is under integration/<run>/. Top-level baseline/recovery logs also capture errors before a test namespace exists.

| Symptom | Inspect / action |
| --- | --- |
| Context/profile refused | Use only gpu-node-selector and Docker colima-gpu-node-selector. Check the creation record and profile identity; do not bypass guards or switch to an unrelated cluster. |
| Nodes unavailable | Run make cluster-up, then make cluster-configure. CPU budgets remain Colima 6 and Minikube 2 per node. |
| Missing/stale images | Run make docker-build minikube-load, then make install deploy. Every node's config digest must match the local native image. |
| Other GPU workloads exist | Inspect kubectl --context gpu-node-selector get aiworkloads,pods -A. Remove only known workloads you own before running the suite. Capacity/allocatable stays 2 while occupied. |
| E2E lock exists | Read work/day-09/active/owner and confirm the process has ended before removing that exact stale lock directory. Do not overlap baseline/recovery runs. |
| Pending with mixed constraints | Inspect PodScheduled and events. GPU shortage plus node selector failures correctly reports MultipleSchedulingConstraints; it is not a classifier failure. |
| Drift/recreation timeout | Inspect injected mutation, stage snapshots and operator logs. Check owner UIDs, desired generation and controller observations. Do not adopt a foreign child. |
| Stability failure | Compare stability-before.json and stability-current.json. The test ignores resourceVersion/managedFields but includes specs, UID sets, generations and full CR status. |
| Recovery timeout | Inspect kubelet-before/after InvocationID and final node/plugin logs. Check the validated GPU worker with Minikube SSH and systemctl status kubelet. Do not restart unrelated nodes. |
| Cleanup failure | The log identifies the generated namespace. Inspect it and its finalizers before retrying deletion of that exact test namespace. Never delete ai-system as test cleanup. |
| Destroy from stopped VM | Colima may have removed its Docker context; the destroy script checks the actual stored context inventory without the environment's synthetic current entry. Minikube can warn about the absent daemon/context while removing its local profile; successful cleanup still requires VM/data-disk/context absence checks. |
| Disk remains after manual Colima delete | Use the guarded project destroy target. Colima 0.10.3 requires --data to remove runtime storage; the project target supplies it and verifies both VM and data-disk paths. Do not manually delete shared Colima directories. |

Routine shutdown is make cluster-down; restart with make cluster-up. Destruction is a separate explicit cluster-destroy operation and is not part of Day 9 testing. On this Mac, race tests require the existing SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk override; retained linker warnings are documented separately from exit status.
