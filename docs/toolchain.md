# Day 1 toolchain record

Day 3 update (24 October 2026): recorded pins are now implemented in go.mod/go.sum and versioned Makefile tool paths. controller-gen v0.22.0 and setup-envtest v0.25.0 were installed locally under bin; Kubernetes 1.37.0 darwin/arm64 envtest assets were downloaded and passed admission tests. Standalone Kustomize is not needed for the current CRD-only installation; the minimal overlay can be rendered with kubectl's embedded v5.8.1. Go builds default to CGO_ENABLED=0 after a host SDK/linker mismatch; pure-Go build/vet/test passed. No host tools were upgraded.

Observed on 20 October 2026, darwin/arm64. Exact command output is retained in ignored `work/day-01/doctor-final.txt`.

Day 7 update, 25 October 2026: added k8s.io/kubelet v0.37.0 and its selected gRPC v1.82.1 dependency; device-plugin v1beta1 registration/allocation passed against the existing Kubernetes v1.37.0 cluster. Native linux/arm64 mock/plugin images built with Go 1.27.1. Runtime security (UID 0, no capabilities/privileged mode for plugin) and restart proof passed. Exact evidence is in work/day-07/. The table below preserves the historical Day 1 inventory.

| Component | Observed / selected version | State |
| --- | --- | --- |
| Homebrew | 7.0.1 | Existing |
| Go | 1.27.1 | Installed; scaffold minimum 1.26.0 |
| Git | 2.50.1 (Apple Git-155) | Existing system tool |
| Colima | 0.10.3 | Installed; not started |
| Docker CLI | 29.8.0 | Installed; daemon not started |
| Buildx | 0.37.1 | Installed; existing symlink became valid |
| Minikube | 1.39.0 | Installed; no cluster |
| kubectl | 1.37.0 | Installed |
| Kubebuilder | 4.16.0 | Installed; go/v4 default inspected |
| jq | 1.7.1-apple | Existing system tool |
| ripgrep | 15.2.0 | Existing Codex-provided PATH tool |
| Make | 3.81 | Existing system tool |
| controller-runtime | v0.25.0 | Selected from installed scaffold; not installed as project dependency yet |
| Kubernetes libraries | v0.37.0 | Selected from controller-runtime v0.25.0 go.mod |
| Kubernetes server / envtest assets | 1.37.0 | Planned; neither installed nor run |
| controller-gen | v0.22.0 | Scaffold pin, deferred to Day 3 |
| Kustomize | v5.8.1 | Scaffold standalone pin; same version reported embedded in kubectl |
| setup-envtest | v0.25.0 | Selected controller-runtime tool version; installation deferred to Day 3 |

The installed Kubebuilder generated an inspection-only scaffold with `--fetch-deps=false` under `work/day-01/scaffold-inspection/`. It declares Go 1.26.0 and controller-runtime v0.25.0. The [release's go.mod](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/go.mod) declares Go 1.26.0 and aligned k8s.io modules v0.37.0. Thus installed Go 1.27.1 satisfies the stated minimum, and planned server/kubectl both use minor 1.37. This verifies the declared dependency relationship; application compilation, envtest asset availability and actual server compatibility remain later milestone tests.

Day 3 must preserve these exact generator/dependency pins in the real Makefile/go.mod/go.sum, resolve setup-envtest at the selected release, and validate assets on darwin/arm64. Do not silently substitute newer independent Kubernetes modules. Confirm Minikube supports the pinned server at Day 2 without creating another profile.

Brewfile describes the tool inventory, not reproducible bottle locking. The installer keeps installed tools and never upgrades unrelated formulas. Future versions may differ on a clean host; run doctor and compare this table before scaffolding. ripgrep currently comes from the app's PATH; `make tools` installs Homebrew ripgrep when run in a shell where it is unavailable. No global shell PATH configuration was modified.

[Kubebuilder's upstream guidance](https://github.com/kubernetes-sigs/kubebuilder) identifies generated go.mod as the dependency source; [controller-runtime compatibility guidance](https://github.com/kubernetes-sigs/controller-runtime) explains its Kubernetes-library minor alignment.
