# ADR 0001: Default scheduler owns placement

Status: Accepted for MVP, 20 October 2026.

Context: Multiple AIWorkloads can compete for two simulated GPUs. An operator-side capacity counter would race with other workloads and duplicate scheduling policy.

Decision: AIWorkload expresses GPU count/product; operator generates requests/limits, required selectors and toleration. The default scheduler selects a node using resource accounting and constraints. Never set nodeName, bind Pods, cap replicas based on inventory or modify scheduler behavior.

Consequences: Excess replicas remain Pending and status explains scheduler observations. Labels identify product eligibility but cannot track resource use. Concurrent allocation remains Kubernetes' responsibility. Test exhaustion and release in a real cluster, since envtest has no scheduler.
