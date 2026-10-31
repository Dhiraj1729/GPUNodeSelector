#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
topology
bash "$ROOT/scripts/images.sh" verify
bash "$ROOT/scripts/plugin.sh" verify
k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
run="$(date -u +%Y%m%dT%H%M%SZ)-$$"
out="${DEMO_EVIDENCE_ROOT:-$ROOT/work/day-08}/$run"
mkdir -p "$out"
ns="$(printf '%s' '{"apiVersion":"v1","kind":"Namespace","metadata":{"generateName":"ai-workloads-day8-"}}' | k create -f - -o jsonpath='{.metadata.name}')"
snapshot() {
  local stage="$1"
  k -n "$ns" get aiworkloads,deployments,services,replicasets,pods -o json > "$out/$stage.json" || return
  k -n "$ns" get events -o json > "$out/$stage-events.json"
}
cleanup() {
  local result=$?
  trap - EXIT
  snapshot final || true
  k -n ai-system logs deployment/aiworkload-operator --tail=150 > "$out/operator.log" 2>&1 || true
  k -n ai-system logs daemonset/fake-gpu-device-plugin --tail=150 > "$out/plugin.log" 2>&1 || true
  k delete namespace "$ns" --wait=true --timeout=120s || result=1
  echo "Evidence: $out (exit $result)"
  exit "$result"
}
trap cleanup EXIT
k get nodes -o json > "$out/nodes.json"
create_workload() {
  jq -n --arg name "$1" --arg ns "$ns" --argjson replicas "$2" '{apiVersion:"ai.dhiraj.io/v1alpha1",kind:"AIWorkload",metadata:{name:$name,namespace:$ns},spec:{image:"gpu-node-selector/mock-inference:dev",replicas:$replicas,port:8080,gpu:{product:"A100",count:1},resources:{cpu:"100m",memory:"128Mi"}}}' | k create -f -
}
running() {
  k -n "$ns" get aiworkload "$1" -o json | jq -e --argjson n "$2" '. as $w | .status.observedGeneration==.metadata.generation and .spec.replicas==$n and .status.phase=="Running" and (.status.readyReplicas//0)==$n and (.status.pendingReplicas//0)==0 and any(.status.conditions[]?; .type=="Ready" and .status=="True" and .observedGeneration==$w.metadata.generation)' >/dev/null
}
patch() { k -n "$ns" patch aiworkload "$1" --type=merge -p "$2"; }
devices() {
  local stage="$1" expected="$2" pod port
  k -n "$ns" get pods -o json > "$out/$stage-pods.json"
  jq -e --arg node "$GPU_NODE" --argjson n "$expected" '[.items[]|select(.metadata.deletionTimestamp==null)] | length==$n and all(.[]; .spec.nodeName==$node and .status.phase=="Running" and any(.status.conditions[]; .type=="Ready" and .status=="True") and .spec.containers[0].resources.requests["ai.dhiraj.io/gpu"]=="1" and .spec.containers[0].resources.limits["ai.dhiraj.io/gpu"]=="1")' "$out/$stage-pods.json" >/dev/null
  : > "$out/$stage-devices.jsonl"
  while read -r pod port; do
    k get --raw "/api/v1/namespaces/$ns/pods/$pod:$port/proxy/device" | jq -c --arg pod "$pod" '.+{pod:$pod}' >> "$out/$stage-devices.jsonl"
  done < <(jq -r '.items[]|select(.metadata.deletionTimestamp==null)|[.metadata.name,(.spec.containers[0].ports[0].containerPort|tostring)]|@tsv' "$out/$stage-pods.json")
  jq -se --argjson n "$expected" 'length==$n and all(.[]; .devices=="fake-gpu-0" or .devices=="fake-gpu-1") and ([.[].devices]|unique|length)==$n' "$out/$stage-devices.jsonl" >/dev/null
}
http_checks() {
  local stage="$1" port="$2" path
  for path in healthz readyz device; do
    k get --raw "/api/v1/namespaces/$ns/services/first:$port/proxy/$path" > "$out/$stage-$path.json"
  done
  jq -e '.status=="ok"' "$out/$stage-healthz.json" >/dev/null
  jq -e '.status=="ready"' "$out/$stage-readyz.json" >/dev/null
  printf '%s' '{"prompt":"day8"}' > "$out/request.json"
  k create --raw "/api/v1/namespaces/$ns/services/first:$port/proxy/inference" -f "$out/request.json" > "$out/$stage-inference.json"
  jq -e '.response=="mock: day8" and (.devices=="fake-gpu-0" or .devices=="fake-gpu-1")' "$out/$stage-inference.json" >/dev/null
}
create_workload first 2
wait_for running first 2
snapshot two
uid="$(k -n "$ns" get aiworkload first -o jsonpath='{.metadata.uid}')"
jq -e --arg uid "$uid" '[.items[]|select(.kind=="Deployment" or .kind=="Service")]|length==2 and all(.[]; any(.metadata.ownerReferences[]; .uid==$uid and .controller==true))' "$out/two.json" >/dev/null
devices two 2
http_checks two 8080
patch first '{"spec":{"replicas":3}}'
exhausted() {
  snapshot exhausted
  jq -e '[.items[]|select(.kind=="Pod" and .metadata.deletionTimestamp==null)] | length==3 and ([.[]|select(.status.phase=="Running")]|length)==2 and ([.[]|select(.status.phase=="Pending" and any(.status.conditions[]?; .type=="PodScheduled" and .status=="False" and .reason=="Unschedulable" and (.message|contains("Insufficient ai.dhiraj.io/gpu"))))]|length)==1' "$out/exhausted.json" >/dev/null || return 1
  # Independently recognize capacity-only evidence; every additional clause is mixed.
  local pure
  pure="$(jq '[.items[]|select(.kind=="Pod")|.status.conditions[]?|select(.type=="PodScheduled" and .status=="False")|.message|split(". preemption:")[0]|sub("^.*nodes are available: *";"")|split(",")[]|gsub("^ +|[. ]+$";"")|sub("^[0-9]+ (node\\(s\\) )?";"")] | all(.[]; .=="Insufficient ai.dhiraj.io/gpu")' "$out/exhausted.json")"
  jq -e --argjson pure "$pure" '.items[]|select(.kind=="AIWorkload")| .status.observedGeneration==.metadata.generation and .spec.replicas==3 and .status.desiredReplicas==3 and .status.readyReplicas==2 and .status.pendingReplicas==1 and (if $pure then .status.phase=="CapacityBlocked" and any(.status.conditions[]; .type=="Ready" and .reason=="InsufficientGPU") and any(.status.conditions[]; .type=="CapacityAvailable" and .status=="False") else .status.phase=="Pending" and any(.status.conditions[]; .type=="Ready" and .reason=="MultipleSchedulingConstraints") and any(.status.conditions[]; .type=="CapacityAvailable" and .status=="Unknown") end)' "$out/exhausted.json" >/dev/null
}
wait_for exhausted
patch first '{"spec":{"replicas":1}}'
wait_for running first 1
snapshot one
create_workload second 1
wait_for running second 1
wait_for running first 1
devices reuse 2
snapshot reuse
k -n "$ns" annotate service first user-note=keep
ip="$(k -n "$ns" get service first -o jsonpath='{.spec.clusterIP}')"
patch first '{"spec":{"port":9090}}'
wait_for running first 1
wait_for running second 1
snapshot port
k -n "$ns" get service first -o json | jq -e --arg ip "$ip" '.spec.clusterIP==$ip and .metadata.annotations["user-note"]=="keep" and .spec.ports[0].port==9090 and .spec.ports[0].targetPort=="http"' >/dev/null
k -n "$ns" get deployment first -o json | jq -e '.spec.template.spec.containers[0]|.ports[0].containerPort==9090 and any(.env[]; .name=="PORT" and .value=="9090") and .livenessProbe.httpGet.port=="http" and .readinessProbe.httpGet.port=="http"' >/dev/null
k -n "$ns" get aiworkload first -o json | jq -e --arg ns "$ns" '.status.endpoint==("http://first."+$ns+".svc:9090")' >/dev/null
http_checks port 9090
devices port 2
second_uid="$(k -n "$ns" get pods -o json | jq -r '.items[]|select(.metadata.annotations["ai.dhiraj.io/workload-name"]=="second")|.metadata.uid')"
# Build/load a distinct tag, including distinct OCI version metadata (new image ID).
tag="day8-$run"
COMPONENTS=mock-inference IMAGE_TAG="$tag" bash "$ROOT/scripts/images.sh" build > "$out/rollout-build.log" 2>&1
COMPONENTS=mock-inference IMAGE_TAG="$tag" bash "$ROOT/scripts/images.sh" load > "$out/rollout-load.log" 2>&1
old="$(k -n "$ns" get pods -o json | jq -r '.items[]|select(.metadata.annotations["ai.dhiraj.io/workload-name"]=="first")|.metadata.uid')"
old_image="$(k -n "$ns" get pods -o json | jq -r --arg old "$old" '.items[]|select(.metadata.uid==$old)|.status.containerStatuses[0].imageID')"
k -n "$ns" get deployment first -o json | jq -e '.spec.strategy.rollingUpdate.maxSurge==0 and .spec.strategy.rollingUpdate.maxUnavailable==1' >/dev/null
patch first "$(jq -n --arg image "gpu-node-selector/mock-inference:$tag" '{spec:{image:$image}}')"
deadline=$((SECONDS+180))
while true; do
  k -n "$ns" get pods -o json | jq -c . >> "$out/rollout-timeline.jsonl"
  running second 1 || fail 'Second workload lost availability during rollout.'
  if running first 1; then break; fi
  (( SECONDS < deadline )) || fail 'Saturated rollout timed out.'
  sleep 1
done
snapshot rollout
# maxSurge limits nonterminating Pods, not the overlap with a draining Pod.
jq -se --arg second "$second_uid" 'all(.[]; ([.items[]|select(.metadata.annotations["ai.dhiraj.io/workload-name"]=="first" and .metadata.deletionTimestamp==null)]|length)<=1 and any(.items[]; .metadata.uid==$second and .metadata.deletionTimestamp==null and any(.status.conditions[]?; .type=="Ready" and .status=="True")))' "$out/rollout-timeline.jsonl" >/dev/null
jq -e --arg old "$old" --arg image "gpu-node-selector/mock-inference:$tag" --arg oldimage "$old_image" '[.items[]|select(.kind=="Pod" and .metadata.annotations["ai.dhiraj.io/workload-name"]=="first")]|length==1 and .[0].metadata.uid!=$old and .[0].spec.containers[0].image==$image and .[0].status.containerStatuses[0].imageID!=$oldimage and (.[0].status.containerStatuses[0].imageID|length)>0' "$out/rollout.json" >/dev/null
devices rollout 2
http_checks rollout 9090
cp "$ROOT/work/day-08/images/"*.json "$out/"
k get nodes -o json | jq -e --arg gpu "$GPU_NODE" --arg r "$RESOURCE" 'all(.items[]; if .metadata.name==$gpu then .status.capacity[$r]=="2" and .status.allocatable[$r]=="2" else .status.capacity[$r]==null end)' >/dev/null
echo 'PASS: two distinct devices, truthful exhaustion, recovery/reuse, HTTP/port update and saturated rollout.'
