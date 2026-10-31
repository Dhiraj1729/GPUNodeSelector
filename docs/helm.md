# Helm deployment

Two charts separate platform lifecycle from application lifecycle:

- `charts/gpu-node-selector`: CRD, single Recreate operator Deployment, ServiceAccount, generated-equivalent RBAC and fake GPU DaemonSet. Fixed platform names deliberately allow only one installation per cluster. The plugin advertises two fake A100 devices by default; `plugin.devices` configures 1–256 simulated devices per GPU node.
- `charts/aiworkloads`: one mock Llama AIWorkload per release. It does not create a Deployment or Service: reconciliation creates those with controller OwnerReferences. The server executes deterministic CPU-only mock inference, not a Llama model.

The API supports image, replicas, port, GPU product/count and CPU/memory requests. Selectors and the exact GPU NoSchedule toleration are derived by the operator; arbitrary selectors/tolerations are not API fields and chart validation rejects unsupported values. GPU requests equal limits. Keep the CPU budgets at Colima 6 and Minikube 2 per node.

## Ordered startup

```sh
make tools doctor
make cluster-up cluster-configure
make docker-build minikube-load
make helm-test
make helm-install
make helm-aiworkloads
make helm-verify
make helm-status
```

Run sequentially from the repository. Helm CLI is included in the tool installer; Helm 4.3.0 was used for local chart tests. Make wrappers always use the validated `gpu-node-selector` context and existing profile ownership guards. They do not start a VM implicitly. Images must already be loaded on every node. Platform installation uses local image IDs as Pod annotations so rebuilding a mutable `dev` tag triggers a rollout. An unchanged image ID does not trigger a restart.

`helm-install` uses `helm upgrade --install`, creates `ai-system` if absent and waits for the platform rollouts. It rejects conflicting ownership, incompatible existing CRDs, competing device-plugin socket mounts and unexpected usable pre-existing GPU capacity. Kubelet may retain the previous device count with allocatable=0 after plugin removal; this inactive state is accepted for reinstall. It never uses `--take-ownership` or automatic forced replacement. A failed install leaves a diagnosable release: inspect `helm-status`, events and logs, fix the cause and retry.

`helm-aiworkloads` uses the fixed `mock-llama` release in `ai-workloads`, with two one-GPU replicas. `helm-inference` is its alias. Helm installation completing does not imply custom-resource readiness: `helm-verify` waits for matching observed generation and desired replica readiness, checks GPU placement/requests/toleration, and calls health, readiness, device and inference endpoints through the Kubernetes API proxy. Zero replicas is a valid scaled-to-zero state and skips HTTP checks. Requests that cannot fit intentionally time out verification rather than reporting success.

## Configuring fake GPU capacity

The platform chart defaults to `plugin.devices: 2`. To change it, edit `charts/gpu-node-selector/values.yaml`, or create an override file:

```yaml
# work/platform-values.yaml
plugin:
  devices: 4
```

After removing GPU workloads and waiting for their Pods to disappear, install the override:

```sh
make helm-install HELM_VALUES=work/platform-values.yaml
kubectl --context gpu-node-selector get nodes \
  -l ai.dhiraj.io/gpu-node=true \
  -o custom-columns='NAME:.metadata.name,CAPACITY:.status.capacity.ai\.dhiraj\.io/gpu,ALLOCATABLE:.status.allocatable.ai\.dhiraj\.io/gpu'
make helm-aiworkloads
make helm-verify
```

Wait for capacity and allocatable to show the chosen count before redeploying workloads. The chart passes the integer as `--devices` to the plugin; the valid range is 1–256. Changing it rolls the plugin Pod. Capacity is shared across namespaces, and each replica consumes its requested GPU count. CPU/memory and node selectors still constrain placement. These are simulated devices, not additional physical GPUs.

Use the remove-workloads/update-plugin/redeploy sequence when changing capacity, especially when reducing it. Keep passing your override on subsequent `helm-install` runs; the Make wrapper also supplies image values, so do not rely on previous release overrides being retained. `HELM_VALUES` applies to the chart targeted by that command: do not pass platform values to `helm-aiworkloads`.

The existing demo, plugin verification, recovery and lifecycle tests assume exactly **two devices**. Before running those tests, remove all GPU workloads, restore `plugin.devices: 2` in the chart defaults, run `make helm-install` without an override, and confirm node capacity is two. Custom-capacity chart rendering and validation are covered by `make helm-test`; the original two-device E2E scenarios are unchanged.

## Values and additional workloads

```sh
# Inspect defaults, copy them to a local file and edit as needed:
helm show values charts/aiworkloads
make helm-aiworkloads HELM_VALUES=/absolute/path/my-workload-values.yaml
make helm-verify

# Deliberately exhaust capacity (two run; the third waits):
make helm-aiworkloads HELM_VALUES=charts/aiworkloads/values-capacity.yaml
make helm-status
kubectl --context gpu-node-selector -n ai-workloads describe aiworkload mock-llama
# Restore the chart defaults:
make helm-aiworkloads
make helm-verify
```

The capacity scenario is expected to fail `helm-verify` readiness. Scheduling diagnosis may combine GPU shortage and selector mismatch (`MultipleSchedulingConstraints`). Do not expect a capacity-only reason on this topology.

Keep a workload's `name` stable across upgrades; changing it deletes the old CR and creates a new one. For multiple workloads, use separate releases and unique names, sharing the same two-device pool:

```sh
helm --kube-context gpu-node-selector upgrade --install mock-llama-small \
  charts/aiworkloads -n ai-workloads --create-namespace \
  --set name=mock-llama-small --set replicas=1
```

Make status/verify/removal targets address only the default workload release. Inspect additional releases with Helm and generation-aware AIWorkload status, then uninstall them explicitly before platform removal. Chart schemas validate configuration locally; Kubernetes admission remains authoritative for quantities such as memory. Platform image values can be configured with direct Helm commands; the Make wrapper intentionally follows the existing local `dev` image workflow.

## Upgrade and ownership

Do not mix `make deploy`/`plugin-deploy` with a Helm platform. Those deployment paths now reject Helm-managed objects. `lifecycle-check` detects Helm ownership and uses `helm-install` after restarting the VM. Remove persistent AIWorkloads before lifecycle and E2E tests; the tests require free devices and create their own temporary resources.

For a previous kubectl installation, `helm-install` refuses adoption. Safest migration for disposable local data is `make cluster-destroy CONFIRM=gpu-node-selector`, followed by the complete startup sequence above. This deletes all project cluster data. To preserve other cluster data, first remove all AIWorkloads and wait for children to disappear, inspect the old platform and its ownership, then deliberately remove only the old operator, plugin and their RBAC before installing Helm. There is no automatic adoption or broad namespace deletion. Do not run a local manager alongside the in-cluster operator.

Helm installs the CRD from `crds/` only when absent. Helm does not upgrade or delete it automatically ([Helm CRD documentation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/)). After changing API definitions:

```sh
make generate manifests helm-sync helm-test
# Review schema changes for compatibility and back up existing AIWorkloads first.
kubectl --context gpu-node-selector diff -f charts/gpu-node-selector/crds/ai.dhiraj.io_aiworkloads.yaml
# Only after reviewing the diff and confirming this is the project's CRD:
kubectl --context gpu-node-selector apply -f charts/gpu-node-selector/crds/ai.dhiraj.io_aiworkloads.yaml
make helm-install
```

`kubectl diff` returns 1 when changes exist. `helm-test` rejects a packaged CRD that differs from the generated source and compares chart RBAC/operator/plugin specifications with the existing manifests. Never delete the CRD as an upgrade step: that deletes every AIWorkload. Chart version is currently 0.1.0; increment it when publishing changed chart packages.

## Removal and resource release

```sh
make helm-aiworkloads-remove  # Uninstall default CR release, wait for child GC
# Explicitly uninstall any additional workload releases before proceeding.
make helm-uninstall          # Refuses while AIWorkloads or GPU-consuming Pods exist
make cluster-down            # Stop VM and release runtime CPU/RAM; retain disk
# Or permanently remove this project's VM, data disk, profiles and contexts:
make cluster-destroy CONFIRM=gpu-node-selector
```

Helm uninstall retains namespaces and the CRD. It does not release Colima's CPU/RAM or delete its disk; use cluster-down/destroy for that. Workload removal waits for Deployments, Services, ReplicaSets and Pods belonging to the removed CRs. Cluster destruction remains the complete cleanup path, including Helm release metadata, while preserving repository files and host tools.

## Verification scope

`make helm-test` performs strict chart linting, CRD synchronization checks, typed rendering comparisons against the established platform manifests, invalid-value rejection and tests that rendered workloads can be consumed by the real operator builders. `make verify` includes these checks. Live Helm installation, upgrade, inference and uninstall must also be exercised on a running project cluster with the ordered commands above; local chart tests alone do not prove those behaviors.
