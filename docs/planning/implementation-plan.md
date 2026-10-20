# Ten-day implementation plan

**Approval:** overall plan and Day 1 implementation approved on 20 October 2026. Day 1 results are recorded in progress.md. Stop at the Day 1 handoff; later-day execution awaits the next user instruction. Day numbers are sequential milestones, not dates. Complete an exit gate before advancing.

Read [environment findings](environment-inspection.md) and [technical decisions](technical-design.md) alongside this plan. All commands below are **future verification contracts**, not claims that targets exist or have run. Commit entries are proposed checkpoint contents/messages; create commits only under the implementation session's authorization.

## Day 1 — Environment, repository, architecture

**Objective:** establish a reproducible development base and lock the responsibility boundaries.

**Files:** `.gitignore`, `Brewfile`, initial `Makefile`, `scripts/doctor.sh`, `scripts/install-tools.sh`, `docs/architecture.md`, `docs/local-cluster.md`, ADRs 0001–0003, `docs/planning/progress.md`.

**Steps:**
1. Recheck applicable instructions, current files, and approval; preserve this planning bundle. Initialize Git after approval.
2. Locate/install missing host tools; inspect Colima profiles, Docker contexts, Minikube profiles, and Kubernetes contexts before provisioning. Record versions and host budgets under `work/`.
3. Select and pin a compatible Kubebuilder/Go/controller-runtime/Kubernetes toolchain using the generated dependency set. Record exact patches; do not independently choose latest versions of Kubernetes modules.
4. Implement doctor and tool installation with explicit failures, safe path handling, and no automatic unrelated upgrades.
5. Document default scheduler ownership, fake device approach, GPU isolation, and proposed 6 CPU/8 GiB VM budget. Ignore `work/`, binaries, coverage, downloaded tools, and local credentials.

**Tests:** run doctor with installed tools and a controlled missing-tool PATH; shell syntax validation. Inspect all existing profile names; do not start any cluster.

**Verify:** `make doctor`; `bash -n scripts/doctor.sh scripts/install-tools.sh`; `git diff --check`.

**Observable result / exit:** tools are callable, exact versions are recorded, profile inventory is known, architecture is documented, and the resource budget is accepted. If installs are blocked, record Day 1 incomplete.

**Risks/decisions:** tool availability, dependency downloads, version compatibility, free RAM. No proof of Kubernetes functionality yet.

**Checkpoint commit:** `chore: establish development tooling and architecture`.

## Day 2 — Three nodes and manual scheduling proof

**Objective:** independently demonstrate default-scheduler GPU capacity accounting before writing the operator/plugin.

**Files:** `scripts/cluster-up.sh`, `scripts/cluster-configure.sh`, `scripts/cluster-down.sh`, `scripts/lib/environment.sh`, `hack/manual-gpu-capacity.sh`, `hack/scheduling-proof/`, Makefile, local-cluster docs.

**Steps:**
1. Create only named project profiles; explicitly select Docker context. Start three Minikube nodes with Docker/containerd and pinned Kubernetes version.
2. Discover control-plane by role label; require two Ready workers. Reuse validated role labels if already configured; otherwise deterministically select workers, then label roles. Reject ambiguous/missing topology. Permit explicit node overrides only after membership checks.
3. Label GPU worker `ai.dhiraj.io/gpu-node=true`, product A100; taint `ai.dhiraj.io/gpu=true:NoSchedule`. Label CPU worker for test selection. Validate control-plane scheduling behavior rather than assuming a Minikube default taint.
4. In isolated diagnostic mode, patch GPU node status capacity to 2; wait for allocatable to become 2. Require plugin absence. Use `/status`, JSON Pointer escaping (`ai.dhiraj.io~1gpu`), and guarded context/node validation.
5. Schedule plain CPU proof pod explicitly selecting the CPU worker; separately verify a pod targeting GPU labels without toleration stays unscheduled. Run three one-GPU standalone proof pods with selector+toleration: two Running, one Pending. Delete a running standalone pod and observe the third run.
6. Remove all proof pods and manually advertised resource; wait for capacity/allocatable removal. Save evidence. Keep diagnostic mode disabled by default.

**Tests:** rerun cluster/configure safely; wrong-context rejection before mutation; topology failure diagnostics; capacity exhaustion/release; missing toleration; stop/start preservation when practical.

**Verify:** `make cluster-up`; `make cluster-configure`; `make scheduling-proof`; `make scheduling-proof-cleanup`; `kubectl --context gpu-node-selector get nodes -o wide`.

**Observable result / exit:** three Ready nodes; CPU/GPU isolation demonstrated; 2 running + 1 pending transitions after deletion; manual capacity fully removed. No fake device env var is expected in this stage.

**Risks/decisions:** nested container resource limits; control-plane may accept ordinary pods; manual capacity is scheduler-only evidence and must never coexist with plugin-managed capacity.

**Checkpoint commit:** `feat: add isolated cluster and scheduling proof`.

## Day 3 — Kubebuilder API and admission contract

**Objective:** make AIWorkload a validated namespaced Kubernetes API.

**Files:** Kubebuilder `PROJECT`, `go.mod`, `go.sum`, generated manager entrypoint (normally `cmd/main.go`), `api/v1alpha1/`, `config/crd/`, `config/samples/`, generated RBAC/default configuration, Makefile.

**Steps:** scaffold without overwriting planning/tooling files; use a temporary staging directory if necessary. Generate group `ai`, domain `dhiraj.io`, version `v1alpha1`, kind `AIWorkload`. Define pointer replicas/port to distinguish omission from zero, required GPU fields, optional native CPU/memory quantities, status subresource, conditions and printer columns. Document validation semantics; generate manifests and deepcopy code. Pin local generator and envtest versions.

**Tests:** envtest API-server defaulting and validation: omitted replicas -> 1; zero replicas accepted; negatives rejected; empty image/product, missing GPU, fractional/zero GPU count and invalid ports rejected; valid/invalid/negative CPU and memory quantities covered. Required GPU/count have no implicit default. Fake clients cannot prove CRD admission behavior.

**Verify:** `make generate manifests`; `make fmt vet test`; `make envtest`; `make install`; `kubectl --context gpu-node-selector explain aiworkload.spec`.

**Observable result / exit:** CRD installed and defaults/rejections demonstrated; generation reproducible. Sample CR may exist without child objects until controller implementation.

**Risks/decisions:** module path confirmation; generated quantity-schema/CEL compatibility; don't generate a webhook merely to add defaults supported by CRD schema.

**Checkpoint commit:** `feat: define AIWorkload API and validation`.

## Day 4 — Desired-state builders and mock server

**Objective:** produce testable workload manifests and a usable CPU-only mock application.

**Files:** `internal/workload/` builders/constants/tests, `cmd/mock-inference/`, `internal/mockinference/`, mock Dockerfile target under `deploy/docker/`, reconciliation docs draft.

**Steps:** build Deployment and ClusterIP Service using stable immutable selectors, name+UID identity, OwnerReferences, GPU limit, product selector, GPU-node toleration, CPU/memory requests, probes, and non-root security. Leave `nodeName` empty. Choose RollingUpdate maxSurge=0/maxUnavailable=1 to permit updates when both GPUs are occupied. Implement `/healthz`, `/readyz`, `/device`, deterministic `POST /inference`, configurable port, signal shutdown, bounded requests/timeouts. Mock starts without GPU env in manual tests.

**Tests:** table-driven builder tests for replicas/image/port/GPU counts/product/resources, selectors, tolerations, ownership, rollout strategy, omitted quantities, explicit zero replicas; HTTP method/payload behavior and device env display.

**Verify:** `make fmt vet test build`; `go test -race ./internal/workload/... ./internal/mockinference/...`.

**Observable result / exit:** builder unit tests pass; mock endpoint tests pass; no Kubernetes-client mutation hidden in builders.

**Risks/decisions:** GPU rollout availability tradeoff; preserve selector identity across updates; Pod probes are HTTP probes rather than shell-dependent container health checks.

**Checkpoint commit:** `feat: build desired workloads and mock inference server`.

## Day 5 — Reconciliation, ownership, updates, drift

**Objective:** reconcile exactly one Deployment and Service safely and repeatedly.

**Files:** `internal/controller/` reconciler and mutation helpers/tests; `config/rbac/`; `docs/reconciliation.md`.

**Steps:** fetch CR, handle NotFound/deletion, validate defensively, reconcile children with deliberate CreateOrPatch field merging, preserve Service allocations/unrelated metadata, reject existing objects not controlled by the CR UID, and return errors. Add owned Deployment/Service watches and Pod-to-CR mapping validated against identity labels/owner chain. Keep operator RBAC free of Node mutations/binding permissions. Record structured outcomes; add no finalizer. Establish basic status write mechanism for Day 6.

**Tests:** envtest create, repeat, scale/image/GPU/port update, child recreation, replicas/limits/selector/toleration drift, unrelated metadata retention, stable clusterIP, same-name foreign/unowned child rejection, stale UID labels, and update conflict retry. Use API defaulted objects to catch perpetual-diff errors.

**Verify:** `make generate manifests fmt vet test envtest`; `make race`.

**Observable result / exit:** one child of each type; repeat reconciliation causes no spec churn; drift converges; owner collision leaves foreign object untouched; user metadata survives.

**Risks/decisions:** CreateOrPatch ownership is explicit code policy, not automatic field-manager enforcement; list merging and defaulted fields can cause update loops. envtest has no Deployment controller, scheduler, or garbage collector.

**Checkpoint commit:** `feat: reconcile owned workloads and repair drift`.

## Day 6 — Status and scheduling diagnostics

**Objective:** provide stable, truthful status driven by Deployment and Pod observations.

**Files:** `internal/controller/status.go`, diagnosis helpers/tests, `docs/status-and-failures.md`, API printer/status updates if needed.

**Steps:** aggregate only pods belonging to the current CR/Deployment, account for terminating/old rollout pods, and gate readiness on observed Deployment generation. Implement documented phase precedence and Ready/Progressing/Scheduled/CapacityAvailable conditions with Unknown where evidence is absent. Classify structured reasons first; isolate bounded scheduler-message parsing for GPU capacity, selector mismatch, taints, and mixed causes. Patch semantically changed status only; preserve condition transition times if status is unchanged. Pod events trigger reconciliation; bounded refresh only for unresolved progress.

**Tests:** table-driven zero replicas, rollout, stale Deployment generation, image pull, crash, progress deadline, capacity-only and mixed scheduling blockers, old pods, readiness changes and condition transitions. Count status patch calls/resource versions across unchanged observations; avoid time-dependent message churn.

**Verify:** `make test envtest race`; `make generate manifests`; review printed columns against status fixtures.

**Observable result / exit:** deterministic classifications with reasons and current/stale generation distinction; no false Running or repeated unchanged status writes. Real capacity status proof follows on Day 8.

**Risks/decisions:** scheduler messages are not a stable structured taxonomy; mixed blockers must not be reported as GPU shortage alone. Pending includes more than unscheduled pods.

**Checkpoint commit:** `feat: report workload status and scheduling blockers`.

## Day 7 — Fake GPU device plugin

**Objective:** implement the kubelet device-plugin protocol with testable allocation and lifecycle behavior.

**Files:** `cmd/fake-device-plugin/`, `internal/fakegpu/`, `config/fake-device-plugin/`, plugin Dockerfile target, `docs/fake-device-plugin.md`.

**Steps:** configure two healthy stable IDs; serve the selected kubelet API over Unix socket; register after server readiness; ListAndWatch inventory; validate Allocate requests and return FAKE_GPU_DEVICES. Reject unknown/duplicate IDs in a request, but allow an identical later RPC retry. Kubelet owns allocation bookkeeping; do not build a second scheduler or permanent reservation map. Handle context cancellation and socket recreation with bounded retry/backoff and re-registration. Mount kubelet socket directory only in the node-scoped DaemonSet; validate required UID/security privileges locally.

**Tests:** zero/invalid counts, deterministic IDs, valid/multiple device responses, unknown/duplicate IDs within/across container requests, repeat RPC, canceled streams, gRPC Unix-socket registration harness, recreated socket and graceful shutdown. Race-test lifecycle paths.

**Verify:** `go test -race ./internal/fakegpu/...`; `make test build`; `make docker-build`; `make minikube-load`; `make plugin-deploy`; `make plugin-verify`.

**Observable result / exit:** only GPU worker advertises capacity and allocatable 2 through the plugin; restart restores registration; device ID env confirmed in one temporary proof pod and proof pod deleted.

**Risks/decisions:** kubelet socket permissions, API compatibility, plugin restart vs kubelet restart are different tests. Confirm diagnostic capacity removed before plugin start.

**Checkpoint commit:** `feat: implement fake GPU device plugin`.

## Day 8 — Integrated operator and inference path

**Objective:** run the complete CR → scheduler → kubelet → mock service path.

**Files:** all three Dockerfile targets, `scripts/images.sh`, deployment/default overlays, sample CRs, `scripts/demo.sh` initial version, Makefile deployment targets.

**Steps:** build architecture-correct operator/plugin/mock images, explicitly load into every Minikube node, use IfNotPresent locally, and validate image presence. Deploy platform components and CRD. Apply two-replica sample and verify distinct per-pod device IDs. Scale 2→3→1 and let a second workload consume released capacity. Update image using a new loaded tag; test rollout with both devices occupied. Query service endpoints; test port changes/probes.

**Tests:** 2/2 Running, one excess Pending, CapacityBlocked/InsufficientGPU, release/reuse, unique simultaneous IDs, endpoint responses, all AI pods on GPU worker, image rollout without extra capacity. Cross-build AMD64 image; runtime support remains unverified unless run on matching/emulated nodes.

**Verify:** `make docker-build minikube-load install deploy`; `make demo-smoke`; `make plugin-verify`.

**Observable result / exit:** plugin-backed happy path, exhaustion, recovery, and saturated rollout work; HTTP device and inference output saved. No manual Node capacity patch remains.

**Risks/decisions:** local image caching; :dev replacements don't trigger rollout; image load must cover all nodes. Avoid calling cross-compilation a validated AMD64 deployment.

**Checkpoint commit:** `feat: integrate local AI workload control plane`.

## Day 9 — End-to-end and recovery suite

**Objective:** automate all acceptance scenarios with bounded condition-based waits and failure evidence.

**Files:** `tests/e2e/`, `scripts/e2e.sh`, fixtures/sample collision and failure cases, troubleshooting draft, Makefile verification targets.

**Steps:** assert profile/node identity before mutation; use a dedicated test namespace and run identity. Execute the full matrix in technical-design.md with deterministic cleanup. Add controller restart, child deletion, image/product/GPU-count update, unsatisfiable product/count, foreign-owner collision, plugin restart and recovery. Test kubelet restart only through a narrowly scoped explicit recovery target after validating the installed Minikube mechanism. Preserve failure diagnostics before cleanup.

**Tests:** all original 17 end-to-end cases; deletion waits for children and pods to disappear and proves replacement consumes resources; unchanged CR stability, simultaneous workloads without operator capacity capping, zero replicas, saturated rollout, old-generation status. Confirm ordinary CPU isolation separately from CPU worker placement.

**Verify:** `make test envtest race`; `make e2e`; `make e2e` again; `make e2e-recovery` (project GPU node only).

**Observable result / exit:** two consecutive baseline e2e passes with no leftovers; recovery evidence; failures produce node/pod/CR/event/log snapshots. Explicitly mark any optional unrun recovery case.

**Risks/decisions:** asynchronous termination/GC/device release, timing flakes, test namespace collision. No arbitrary long sleeps or active reference-cluster mutation.

**Checkpoint commit:** `test: cover scheduling lifecycle drift and recovery`.

## Day 10 — Demo, documentation, final verification

**Objective:** make the project reproducible and explainable from a fresh checkout.

**Files:** README, all required architecture/reconciliation/plugin/local-cluster/status/demo/troubleshooting docs, ADRs, final Make targets/scripts, progress log.

**Steps:** align docs with actual tested commands/versions; implement full `make demo` and `make verify`; check source generation reproducibility, RBAC, image users/architectures and cleanup guards. Run final sequence, record evidence under work, and document expected limitations. Test stop/start with `cluster-down` preserving profile data. Keep destructive profile deletion separate and explicit. Record future extensions without implementing them.

**Tests:** final verify and demo from clean project state, repeat demo, safe wrong-context refusal, documented stop/start recovery, generated manifests unchanged after regeneration. Fresh checkout rehearsal must preserve planning docs and avoid deleting live work.

**Verify:** `make verify`; `make demo`; `make cluster-down`; `make cluster-up`; `make deploy`; `make demo`.

**Observable result / exit:** all acceptance cases evidenced, coherent version pins, no unexplained failing/skipped checks, documented limitations and clean generated diff. No real GPU/CUDA/LLM or excluded subsystem introduced.

**Risks/decisions:** difference between repeat demo and truly fresh provisioning; record which was actually performed. Project completion requires acceptance results, not just documents.

**Checkpoint commit:** `docs: finalize reproducible demo and verification guide`.

## Assumptions to confirm with plan approval

1. Ten milestones are suitable; no fixed calendar deadlines or daily time budget supplied.
2. Dedicated 6 vCPU/8 GiB/40 GiB Colima VM, three 2 vCPU/2 GiB node budgets are acceptable starting values, subject to Day 1 inspection. Other profiles will not be stopped automatically.
3. Proposed Go module `github.com/Dhiraj1729/GpuNodeSelector` is a local scaffold choice only; approval does not create/publish a remote repository. Change it before scaffolding if desired.
4. Accept zero-surge RollingUpdate with maxUnavailable=1; single-replica updates can briefly interrupt service.
5. CPU/memory fields represent requests; GPU count becomes equal extended-resource request/limit. No public advanced scheduling knobs in v1alpha1.
6. CPU placement test selects CPU worker; separate negative test proves GPU taint isolation. No reliance on an assumed control-plane taint.
7. Standard controller watches across namespaces are allowed with minimum resource verbs; no Node write/bind RBAC. Workloads are demonstrated in ai-workloads.
8. Host tools may need installation and downloads after approval. Exact versions remain a Day 1 gate because no installed compatibility set was available to inspect.

Approving the plan permits starting Day 1; it is not a request to publish, push, or run all ten days automatically. Record the user's actual approval and scope in progress.md.
