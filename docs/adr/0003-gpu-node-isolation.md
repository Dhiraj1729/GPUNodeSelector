# ADR 0003: Isolate the dedicated GPU worker

Status: Accepted for MVP, 20 October 2026.

Context: Labels constrain AI placement but do not prevent ordinary workloads from consuming a GPU node's CPU and memory.

Decision: Bootstrap labels one validated worker gpu-node=true and gpu-product=A100 under ai.dhiraj.io and taints it `ai.dhiraj.io/gpu=true:NoSchedule`. Operator workloads and the device-plugin DaemonSet receive the exact toleration. Plugin runs only on gpu-node=true. A separate worker remains available for CPU applications.

Consequences: Toleration permits placement; product selector and resource requests constrain it. The operator never labels or taints nodes. NoSchedule does not evict existing pods, and explicitly tolerating applications can still use the node; this is scheduling isolation, not a security boundary. CPU-placement and missing-toleration tests are distinct. Do not assume control-plane scheduling behavior.
