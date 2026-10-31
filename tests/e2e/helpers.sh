#!/usr/bin/env bash
# Sourced by scripts/e2e.sh after strict mode, topology and evidence setup.
set -euo pipefail
platform_guard() {
  k -n ai-system get deployment aiworkload-operator -o json | jq -e '.metadata.labels["app.kubernetes.io/part-of"]=="gpu-node-selector" and .spec.replicas==1 and .spec.template.spec.serviceAccountName=="aiworkload-operator"' >/dev/null || fail 'Unexpected operator identity.'
  k -n ai-system get daemonset fake-gpu-device-plugin -o json | jq -e '.metadata.labels["app.kubernetes.io/part-of"]=="gpu-node-selector" and .spec.template.spec.nodeSelector["ai.dhiraj.io/gpu-node"]=="true"' >/dev/null || fail 'Unexpected plugin identity.'
}
no_other_consumers() {
  k get pods -A -o json | jq -e --arg ns "$ns" --arg r "$RESOURCE" 'all(.items[]; .metadata.namespace==$ns or .status.phase=="Succeeded" or .status.phase=="Failed" or all(.spec.containers[]; (.resources.requests[$r]//"0")=="0" and (.resources.limits[$r]//"0")=="0"))' >/dev/null || fail 'Other GPU workloads exist; remove only your known workloads before E2E.'
}
create_workload() {
  jq -n --arg ns "$ns" --arg name "$1" --argjson n "$2" '{apiVersion:"ai.dhiraj.io/v1alpha1",kind:"AIWorkload",metadata:{namespace:$ns,name:$name},spec:{image:"gpu-node-selector/mock-inference:dev",replicas:$n,port:8080,gpu:{product:"A100",count:1},resources:{cpu:"100m",memory:"128Mi"}}}' > "$out/$1-input.json"
  k create --save-config -f "$out/$1-input.json"
}
patch_workload() { k -n "$ns" patch aiworkload "$1" --type=merge -p "$2"; }
running() {
  k -n "$ns" get aiworkload "$1" -o json | jq -e --argjson n "$2" '. as $w | .status.observedGeneration==.metadata.generation and .spec.replicas==$n and .status.phase=="Running" and (.status.readyReplicas//0)==$n and (.status.pendingReplicas//0)==0 and all(.status.conditions[]; .observedGeneration==$w.metadata.generation) and any(.status.conditions[]; .type=="Ready" and .status=="True")' >/dev/null
}
assert_json() {
  local resource="$1" name="$2" filter="$3"
  k -n "$ns" get "$resource" "$name" -o json | jq -e "$filter" >/dev/null
}
gone() { [[ -z "$(k -n "$ns" get "$1" "$2" --ignore-not-found -o name)" ]]; }
owned_pods_gone() {
  k -n "$ns" get pods -o json | jq -e --arg id "$1" 'all(.items[]; .metadata.labels["ai.dhiraj.io/workload-id"]!=$id)' >/dev/null
}
delete_workload() {
  local name="$1" identity oldpods
  identity="$(k -n "$ns" get deployment "$name" -o jsonpath='{.spec.selector.matchLabels.ai\.dhiraj\.io/workload-id}')"
  oldpods="$(k -n "$ns" get pods -l "ai.dhiraj.io/workload-id=$identity" -o json | jq '[.items[].metadata.uid]')"
  k -n "$ns" delete aiworkload "$name" --wait=true --timeout=120s
  wait_for gone deployment "$name"
  wait_for gone service "$name"
  wait_for owned_pods_gone "$identity"
  k -n "$ns" get pods -o json | jq -e --argjson old "$oldpods" 'all(.items[]; .metadata.uid as $uid | $old | index($uid)==null)' >/dev/null
  echo "PASS: garbage collection of $name and all captured Pod UIDs."
}
device_count() {
  local name="$1" count="$2" pod
  pod="$(k -n "$ns" get pods -o json | jq -er --arg name "$name" '[.items[]|select(.metadata.annotations["ai.dhiraj.io/workload-name"]==$name and .metadata.deletionTimestamp==null)]|select(length==1)|.[0].metadata.name')"
  k get --raw "/api/v1/namespaces/$ns/pods/$pod:8080/proxy/device" > "$out/$name-device-$count.json"
  jq -e --argjson n "$count" '.devices|split(",")|length==$n and (unique|length)==$n and all(.[]; .=="fake-gpu-0" or .=="fake-gpu-1")' "$out/$name-device-$count.json" >/dev/null
}
blocked() {
  local name="$1" evidence="$2"
  k -n "$ns" get pods -o json > "$out/$name-blocked-pods.json"
  jq -e --arg name "$name" --arg evidence "$evidence" 'any(.items[]; .metadata.annotations["ai.dhiraj.io/workload-name"]==$name and .metadata.deletionTimestamp==null and .status.phase=="Pending" and any(.status.conditions[]?; .type=="PodScheduled" and .status=="False" and .reason=="Unschedulable" and (.message|contains($evidence))))' "$out/$name-blocked-pods.json" >/dev/null || return 1
  k -n "$ns" get aiworkload "$name" -o json > "$out/$name-blocked-cr.json"
  jq -e '.status.observedGeneration==.metadata.generation and (.status.phase=="Pending" or .status.phase=="CapacityBlocked") and .status.pendingReplicas>=1 and any(.status.conditions[]; .type=="Ready" and .status=="False")' "$out/$name-blocked-cr.json" >/dev/null
}
