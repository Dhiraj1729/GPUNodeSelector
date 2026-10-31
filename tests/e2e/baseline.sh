#!/usr/bin/env bash
set -euo pipefail
# Cases 1-4: assert labels/taint in addition to Day 8 topology/capacity proof.
k get node "$GPU_NODE" -o json | jq -e '.metadata.labels["ai.dhiraj.io/gpu-node"]=="true" and .metadata.labels["ai.dhiraj.io/gpu-product"]=="A100" and any(.spec.taints[]; .key=="ai.dhiraj.io/gpu" and .value=="true" and .effect=="NoSchedule")' >/dev/null
# Case 5: CPU placement and a separate negative GPU isolation test.
for role in cpu gpu; do
  node="$CPU_NODE"; [[ "$role" == cpu ]] || node="$GPU_NODE"
  jq -n --arg node "$node" --arg name "isolation-$role" '{apiVersion:"v1",kind:"Pod",metadata:{name:$name},spec:{nodeSelector:{"kubernetes.io/hostname":$node},containers:[{name:"mock",image:"gpu-node-selector/mock-inference:dev",imagePullPolicy:"IfNotPresent"}]}}' | k -n "$ns" create -f -
done
k -n "$ns" wait --for=condition=Ready pod/isolation-cpu --timeout=180s
assert_json pod isolation-cpu ".spec.nodeName==\"$CPU_NODE\" and (.spec.tolerations|all(.[]; .key!=\"ai.dhiraj.io/gpu\"))"
wait_for assert_json pod isolation-gpu '.spec.nodeName==null and any(.status.conditions[]?; .type=="PodScheduled" and .status=="False" and .reason=="Unschedulable" and (.message|contains("untolerated taint")))'
snapshot isolation
k -n "$ns" delete pod isolation-cpu isolation-gpu --wait=true --timeout=120s

create_workload primary 1
wait_for running primary 1
device_count primary 1
# Cases 15/16: repair each owned field independently, retain unrelated metadata.
k -n "$ns" annotate deployment primary user-note=keep
k -n "$ns" annotate service primary user-note=keep
service_ip="$(k -n "$ns" get service primary -o jsonpath='{.spec.clusterIP}')"
for field in replicas gpu selector toleration; do
  case "$field" in
    replicas) change='{"spec":{"replicas":0}}'; predicate='.spec.replicas==1' ;;
    gpu) change='{"spec":{"template":{"spec":{"containers":[{"name":"inference","resources":{"requests":{"ai.dhiraj.io/gpu":"2"},"limits":{"ai.dhiraj.io/gpu":"2"}}}]}}}}'; predicate='.spec.template.spec.containers[0].resources.requests["ai.dhiraj.io/gpu"]=="1" and .spec.template.spec.containers[0].resources.limits["ai.dhiraj.io/gpu"]=="1"' ;;
    selector) change='{"spec":{"template":{"spec":{"nodeSelector":{"ai.dhiraj.io/gpu-product":"H100"}}}}}'; predicate='.spec.template.spec.nodeSelector["ai.dhiraj.io/gpu-product"]=="A100"' ;;
    toleration) change='{"spec":{"template":{"spec":{"tolerations":[]}}}}'; predicate='any(.spec.template.spec.tolerations[]?; .key=="ai.dhiraj.io/gpu" and .value=="true" and .operator=="Equal" and .effect=="NoSchedule")' ;;
  esac
  k -n "$ns" patch deployment primary --type=strategic -p "$change" -o json > "$out/drift-$field-injected.json"
  # Ensure the mutation reached the API, so a no-op cannot count as repair.
  if jq -e "$predicate" "$out/drift-$field-injected.json" >/dev/null; then fail "Drift $field was not injected."; fi
  wait_for assert_json deployment primary "$predicate"
  wait_for running primary 1
  snapshot "drift-$field-repaired"
done
assert_json deployment primary '.metadata.annotations["user-note"]=="keep"'
assert_json service primary ".metadata.annotations[\"user-note\"]==\"keep\" and .spec.clusterIP==\"$service_ip\""

# Compare semantic state, generations, UID sets and condition transition times;
# API managedFields/resourceVersion may change for unrelated controllers.
stable_state() {
  k -n "$ns" get aiworkload,deployment,service,replicaset,pod -o json | jq -S '[.items[]|{kind,uid:.metadata.uid,generation:.metadata.generation,spec,status:(if .kind=="AIWorkload" then .status else null end)}]|sort_by(.uid)'
}
stable_state > "$out/stability-before.json"
k apply -f "$out/primary-input.json"
deadline=$((SECONDS+12))
while (( SECONDS < deadline )); do
  stable_state > "$out/stability-current.json"
  cmp "$out/stability-before.json" "$out/stability-current.json" || fail 'Unchanged CR caused spec/status/identity churn.'
  sleep 2
done

# Recreate each child using owner watches, with a new UID and same CR ownership.
cr_uid="$(k -n "$ns" get aiworkload primary -o jsonpath='{.metadata.uid}')"
for resource in service deployment; do
  old_uid="$(k -n "$ns" get "$resource" primary -o jsonpath='{.metadata.uid}')"
  k -n "$ns" delete "$resource" primary --wait=true --timeout=120s
  wait_for assert_json "$resource" primary ".metadata.uid!=\"$old_uid\" and any(.metadata.ownerReferences[]; .uid==\"$cr_uid\" and .controller==true)"
  wait_for running primary 1
  snapshot "recreated-$resource"
done

# CR update response contains old status: it must not represent the new generation.
k -n "$ns" patch aiworkload primary --type=merge -p '{"spec":{"gpu":{"product":"H100"}}}' -o json > "$out/old-generation.json"
jq -e '. as $w | .status.observedGeneration<.metadata.generation and all(.status.conditions[]; .observedGeneration<$w.metadata.generation)' "$out/old-generation.json" >/dev/null
wait_for blocked primary 'node affinity/selector'
jq -e '.status.phase=="Pending" and any(.status.conditions[]; .type=="CapacityAvailable" and .status=="Unknown")' "$out/primary-blocked-cr.json" >/dev/null
patch_workload primary '{"spec":{"gpu":{"product":"A100","count":2}}}'
wait_for running primary 1
device_count primary 2
assert_json deployment primary '.spec.template.spec.containers[0].resources|.requests["ai.dhiraj.io/gpu"]=="2" and .limits["ai.dhiraj.io/gpu"]=="2"'
# InvalidImageName is deterministic and avoids relying on an external registry.
patch_workload primary '{"spec":{"image":"INVALID IMAGE"}}'
wait_for assert_json aiworkload primary '.status.observedGeneration==.metadata.generation and .status.phase=="Degraded" and any(.status.conditions[]; .type=="Ready" and .reason=="ImagePullFailure")'
snapshot invalid-image
patch_workload primary '{"spec":{"image":"gpu-node-selector/mock-inference:dev"}}'
wait_for running primary 1
device_count primary 2
patch_workload primary '{"spec":{"gpu":{"count":3}}}'
wait_for blocked primary 'Insufficient ai.dhiraj.io/gpu'
assert_json deployment primary '.spec.replicas==1 and .spec.template.spec.containers[0].resources.requests["ai.dhiraj.io/gpu"]=="3"'
patch_workload primary '{"spec":{"gpu":{"count":1},"replicas":0}}'
wait_for running primary 0
wait_for assert_json aiworkload primary 'any(.status.conditions[]; .type=="Ready" and .reason=="ScaledToZero") and any(.status.conditions[]; .type=="CapacityAvailable" and .status=="Unknown" and .reason=="NoDemand")'
identity="$(k -n "$ns" get deployment primary -o jsonpath='{.spec.selector.matchLabels.ai\.dhiraj\.io/workload-id}')"
wait_for owned_pods_gone "$identity"
patch_workload primary '{"spec":{"replicas":2}}'
wait_for running primary 2
# Cases 13-14: wait for real GC, then consume both devices again.
delete_workload primary
create_workload replacement 2
wait_for running replacement 2
snapshot reuse-after-deletion
delete_workload replacement

# A foreign controller-owned Deployment and an unowned Service are both collisions.
k -n "$ns" create configmap foreign-owner --from-literal=owner=test
foreign_uid="$(k -n "$ns" get configmap foreign-owner -o jsonpath='{.metadata.uid}')"
jq -n --arg uid "$foreign_uid" '{apiVersion:"apps/v1",kind:"Deployment",metadata:{name:"collision-deployment",ownerReferences:[{apiVersion:"v1",kind:"ConfigMap",name:"foreign-owner",uid:$uid,controller:true}]},spec:{replicas:0,selector:{matchLabels:{foreign:"true"}},template:{metadata:{labels:{foreign:"true"}},spec:{containers:[{name:"mock",image:"gpu-node-selector/mock-inference:dev"}]}}}}' | k -n "$ns" create -f -
printf '%s' '{"apiVersion":"v1","kind":"Service","metadata":{"name":"collision-service"},"spec":{"ports":[{"port":81}],"selector":{"foreign":"true"}}}' | k -n "$ns" create -f -
for kind in deployment service; do
  name="collision-$kind"
  k -n "$ns" get "$kind" "$name" -o json | jq -S '{uid:.metadata.uid,owners:.metadata.ownerReferences,spec}' > "$out/$name-before.json"
  create_workload "$name" 0
  wait_for assert_json aiworkload "$name" '.status.observedGeneration==.metadata.generation and .status.phase=="Failed" and any(.status.conditions[]; .type=="Ready" and .reason=="OwnershipConflict")'
  k -n "$ns" get "$kind" "$name" -o json | jq -S '{uid:.metadata.uid,owners:.metadata.ownerReferences,spec}' > "$out/$name-after.json"
  cmp "$out/$name-before.json" "$out/$name-after.json" || fail 'Foreign child was changed.'
done
snapshot collisions
