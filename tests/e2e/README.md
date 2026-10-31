# Day 9 acceptance and recovery

Run `make e2e` twice sequentially, then `make e2e-recovery`. Requires the project cluster, configured nodes, installed CRD and deployed operator/plugin, all three native dev images in Docker and every node, and no other active GPU consumers. These targets do not create or destroy profiles. Never run the local manager concurrently.

The baseline composes the existing Day 8 integration script with the Day 9 lifecycle cases. Each run has its own evidence directory under work/day-09 and server-generated test namespaces. A project-local lock refuses overlapping E2E runs. All waits are bounded; the 12-second stability observation polls for forbidden changes every two seconds. EXIT captures diagnostics before deleting only generated namespaces; cleanup errors fail the run. Fixtures are generated through jq and their submitted CR JSON is retained in the evidence directory.

## Original acceptance matrix

| Case | Assertion / implementation |
| --- | --- |
| 1 | Three Ready nodes, one control-plane and distinct CPU/GPU workers; environment topology helper |
| 2–3 | GPU resource capacity/allocatable exactly 2 only on GPU worker; Day 8 integration/plugin proof |
| 4 | GPU node/product labels and Equal NoSchedule taint; baseline.sh |
| 5 | CPU-selected ordinary Pod Ready; separate GPU-selected Pod without toleration remains unscheduled with taint evidence; baseline.sh |
| 6–8 | Two ready replicas on GPU worker, request=limit=1 and distinct allocated IDs; Day 8 integration |
| 9–10 | Third replica Pending with raw GPU-shortage evidence and generation-aware truthful classification; Day 8 integration |
| 11–12 | Scale to one and second CR consumes released device, without operator replica capping; Day 8 integration |
| 13–14 | Delete CR, wait for Deployment/Service and all captured Pod UIDs to disappear, then another two-replica CR runs; baseline.sh |
| 15 | Independently inject and repair replicas, GPU request/limit, product selector and toleration drift; preserve unrelated metadata/Service IP; baseline.sh |
| 16 | Reapply identical CR and compare specs, object UID sets, generations and full CR status/condition timestamps throughout observation; baseline.sh |
| 17 | Service health/readiness/device/inference responses; Day 8 integration |

Case 10 retains the approved Day 8 refinement: mixed GPU-shortage/selector evidence means Pending/MultipleSchedulingConstraints with CapacityAvailable=Unknown. Exclusively GPU-shortage evidence requires CapacityBlocked/InsufficientGPU. Tests must not relabel mixed evidence just to match the original shorthand.

Additional baseline checks: port update and Service preservation; unique-image saturated rollout; recreate deleted Service/Deployment with new UID and correct owner; old status generation immediately after CR update; unsatisfiable product and GPU count then recovery; successful two-device allocation to one container; deterministic InvalidImageName diagnosis and recovery without registry dependencies; scale-to-zero conditions; foreign-controller Deployment and unowned Service collision preservation.

## Recovery target

`make e2e-recovery` deliberately restarts the project operator, plugin and GPU-worker kubelet. It verifies operator replacement without replacing the unchanged inference Pod, reconciliation after restart, new allocations after plugin restart, and existing/new distinct devices plus deletion/reuse after kubelet restart.

Before kubelet restart it validates the named profile, Docker/context identity, Ready three-node topology, project platform ownership, absence of outside GPU consumers, worker membership in Minikube config, SSH hostname and active loaded kubelet systemd unit. Only `minikube --profile gpu-node-selector ssh --node <validated GPU worker> -- sudo systemctl restart kubelet` changes the service. A changed systemd InvocationID proves a real restart; a fresh allocated Pod proves registration/allocation recovered. No control-plane or CPU-worker kubelet restart, Node capacity patches or profile deletion.

Evidence includes submitted CRs, mutations, stage snapshots, raw scheduling conditions, events, node state, logs, device responses, stability comparisons, kubelet service invocation IDs and exit-code. The nested integration proof retains its own images/rollout/HTTP evidence. On failure see docs/troubleshooting.md. Day 10 final demo/verify orchestration remains a separate milestone.
