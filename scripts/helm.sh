#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
command -v helm >/dev/null || fail 'Helm is missing; run make tools.'
action="${1:-}"
h() { helm --kube-context "$KUBE_CONTEXT" "$@"; }
platform=gpu-node-selector
release=mock-llama
ns=ai-workloads
if [[ -n "${HELM_VALUES:-}" ]]; then
  [[ -f "$HELM_VALUES" ]] || fail 'HELM_VALUES must name an existing values file.'
fi
workloads() { k -n "$ns" get aiworkloads -l "app.kubernetes.io/instance=$release" -o json; }
ready() {
  workloads | jq -e '.items | length>0 and all(.[]; . as $w | .status.observedGeneration==.metadata.generation and (.status.readyReplicas//0)==.spec.replicas and (.spec.replicas==0 or any(.status.conditions[]?; .type=="Ready" and .status=="True" and .observedGeneration==$w.metadata.generation)))' >/dev/null
}
case "$action" in
  lint)
    helm lint --strict "$ROOT/charts/gpu-node-selector" "$ROOT/charts/aiworkloads"
    cmp "$ROOT/config/crd/bases/ai.dhiraj.io_aiworkloads.yaml" "$ROOT/charts/gpu-node-selector/crds/ai.dhiraj.io_aiworkloads.yaml" || fail 'Chart CRD is stale; run make helm-sync.'
    ;;
  install)
    topology
    bash "$ROOT/scripts/images.sh" verify
    existing_crd="$(k get crd aiworkloads.ai.dhiraj.io --ignore-not-found -o json)"
    if [[ -n "$existing_crd" ]]; then
      desired_crd="$(k create --dry-run=client -f "$ROOT/charts/gpu-node-selector/crds/ai.dhiraj.io_aiworkloads.yaml" -o json)"
      jq -e --argjson desired "$desired_crd" '(.spec | del(.conversion,.preserveUnknownFields)) == ($desired.spec | del(.conversion,.preserveUnknownFields))' <<<"$existing_crd" >/dev/null || fail 'Existing CRD schema differs; review the explicit CRD upgrade procedure in docs/helm.md.'
    fi
    # Never adopt kubectl-managed resources or another Helm release.
    platform_args=(-n ai-system)
    if [[ -n "${HELM_VALUES:-}" ]]; then
      platform_args+=(-f "$HELM_VALUES")
    fi
    rendered="$(mktemp)"
    trap 'rm -f "$rendered"' EXIT
    h template "$platform" "$ROOT/charts/gpu-node-selector" "${platform_args[@]}" > "$rendered"
    k get -f "$rendered" --ignore-not-found -o json | jq -es 'all(.[]; (if .kind=="List" then .items[] else . end) | .metadata.labels["app.kubernetes.io/managed-by"]=="Helm" and .metadata.annotations["meta.helm.sh/release-name"]=="gpu-node-selector" and .metadata.annotations["meta.helm.sh/release-namespace"]=="ai-system")' >/dev/null || fail 'Existing platform is not owned by this Helm release. See docs/helm.md migration.'
    k get pods -A -o json | jq -e 'all(.items[]; (.metadata.namespace=="ai-system" and .metadata.labels.app=="fake-gpu-device-plugin") or all(.spec.volumes[]?; (.hostPath.path // "" | startswith("/var/lib/kubelet/device-plugins"))|not))' >/dev/null || fail 'Another device plugin exists.'
    k get nodes -l ai.dhiraj.io/gpu-node=true -o json | jq -e --arg gpu "$GPU_NODE" '.items|length==1 and .[0].metadata.name==$gpu and .[0].metadata.labels["ai.dhiraj.io/gpu-product"]=="A100"' >/dev/null || fail 'Run make cluster-configure first.'
    if [[ -z "$(k -n ai-system get daemonset fake-gpu-device-plugin --ignore-not-found -o name)" ]]; then
      k get nodes -o json | jq -e --arg r "$RESOURCE" --arg gpu "$GPU_NODE" -f "$ROOT/scripts/lib/inactive-gpu-capacity.jq" >/dev/null || fail 'Unexpected usable GPU capacity without a plugin; inspect before installing. After uninstall, wait for kubelet to mark old devices unavailable.'
    fi
    h upgrade --install "$platform" "$ROOT/charts/gpu-node-selector" "${platform_args[@]}" --create-namespace --wait --timeout 180s \
      --set-string "operator.imageID=$(docker --context "$DOCKER_CONTEXT" image inspect gpu-node-selector/operator:dev --format '{{.Id}}')" \
      --set-string "plugin.imageID=$(docker --context "$DOCKER_CONTEXT" image inspect gpu-node-selector/fake-device-plugin:dev --format '{{.Id}}')"
    k wait --for=condition=Established crd/aiworkloads.ai.dhiraj.io --timeout=60s
    k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
    k -n ai-system rollout status daemonset/fake-gpu-device-plugin --timeout=180s
    ;;
  aiworkloads)
    topology
    h status "$platform" -n ai-system >/dev/null
    k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
    # Bash 3.2 treats an empty array as unbound under set -u. Keep the
    # command array nonempty, including when no values override is supplied.
    args=(upgrade --install "$release" "$ROOT/charts/aiworkloads" -n "$ns" --create-namespace --wait --timeout 180s)
    if [[ -n "${HELM_VALUES:-}" ]]; then
      args+=(-f "$HELM_VALUES")
    fi
    h "${args[@]}"
    echo 'AIWorkload release installed. Run make helm-verify for generation-aware readiness and HTTP checks.'
    ;;
  verify)
    topology
    h status "$platform" -n ai-system
    h status "$release" -n "$ns"
    wait_for ready
    items="$(workloads)"
    while read -r name port replicas count product; do
      [[ "$replicas" != 0 ]] || continue
      # Chart names are DNS-1035 labels, so child names match the CR name.
      for path in healthz readyz device; do
        k get --raw "/api/v1/namespaces/$ns/services/$name:$port/proxy/$path" | jq -e 'if has("status") then .status=="ok" or .status=="ready" else (.devices|length)>0 end'
      done
      printf '%s' '{"prompt":"helm verification"}' | k create --raw "/api/v1/namespaces/$ns/services/$name:$port/proxy/inference" -f - | jq -e '.response=="mock: helm verification" and (.devices|length)>0'
      k -n "$ns" get pods -o json | jq -e --arg name "$name" --arg gpu "$GPU_NODE" --argjson n "$replicas" --arg count "$count" --arg product "$product" '[.items[]|select(.metadata.annotations["ai.dhiraj.io/workload-name"]==$name and .metadata.deletionTimestamp==null)] | length==$n and all(.[]; .spec.nodeName==$gpu and .spec.nodeSelector["ai.dhiraj.io/gpu-node"]=="true" and .spec.nodeSelector["ai.dhiraj.io/gpu-product"]==$product and .spec.containers[0].resources.requests["ai.dhiraj.io/gpu"]==$count and .spec.containers[0].resources.limits["ai.dhiraj.io/gpu"]==$count and any(.spec.tolerations[]; .key=="ai.dhiraj.io/gpu" and .operator=="Equal" and .value=="true" and .effect=="NoSchedule"))' >/dev/null
    done < <(jq -r '.items[]|[.metadata.name,.spec.port,.spec.replicas,.spec.gpu.count,.spec.gpu.product]|@tsv' <<<"$items")
    echo 'PASS: Helm workloads reconciled, placed on GPU worker, and served mock inference.'
    ;;
  status)
    cluster
    h list -n ai-system
    h list -n "$ns"
    k -n ai-system get deployments,daemonsets,pods
    k -n "$ns" get aiworkloads,deployments,services,pods
    ;;
  aiworkloads-remove)
    cluster
    names="$(workloads | jq -c '[.items[].metadata.name]')"
    h uninstall "$release" -n "$ns" --ignore-not-found --wait --timeout 180s
    gone() {
      k -n "$ns" get deployments,services,replicasets,pods -o json | jq -e --argjson names "$names" 'all(.items[]; .metadata.annotations["ai.dhiraj.io/workload-name"] as $n | ($names|index($n))==null)' >/dev/null
    }
    wait_for gone
    echo 'PASS: workload release and operator-created children removed.'
    ;;
  uninstall)
    cluster
    k get aiworkloads -A -o json | jq -e '.items|length==0' >/dev/null || fail 'Remove all AIWorkloads before uninstalling the platform.'
    k get pods -A -o json | jq -e --arg r "$RESOURCE" 'all(.items[]; all(.spec.containers[]; ((.resources.requests[$r]//"0")|tonumber)==0))' >/dev/null || fail 'GPU consumers still exist; wait for cleanup before removing the plugin.'
    h uninstall "$platform" -n ai-system --ignore-not-found --wait --timeout 180s
    echo 'Platform removed. CRD and namespaces retained; cluster-destroy removes the entire project cluster.'
    ;;
  *) fail 'Usage: helm.sh lint|install|aiworkloads|verify|status|aiworkloads-remove|uninstall' ;;
esac
