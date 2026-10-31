#!/usr/bin/env bash
set -euo pipefail
platform_guard
no_other_consumers
create_workload survivor 1
wait_for running survivor 1
device_count survivor 1
snapshot before-recovery
# Operator replacement must not replace an unchanged workload Pod.
survivor_uid="$(k -n "$ns" get pods -o json | jq -er '.items[0].metadata.uid')"
operator_uid="$(k -n ai-system get pods -l app=aiworkload-operator -o json | jq -er '.items[]|select(.metadata.deletionTimestamp==null)|.metadata.uid')"
k -n ai-system rollout restart deployment/aiworkload-operator
k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
k -n ai-system get pods -l app=aiworkload-operator -o json | jq -e --arg old "$operator_uid" 'all(.items[]; .metadata.uid!=$old)' >/dev/null
wait_for running survivor 1
k -n "$ns" get pods -o json | jq -e --arg uid "$survivor_uid" '.items|length==1 and .[0].metadata.uid==$uid' >/dev/null
patch_workload survivor '{"spec":{"replicas":2}}'
wait_for running survivor 2
patch_workload survivor '{"spec":{"replicas":1}}'
wait_for running survivor 1
snapshot operator-recovered
bash "$ROOT/scripts/plugin.sh" restart
create_workload after-plugin 1
wait_for running after-plugin 1
device_count after-plugin 1
wait_for running survivor 1
device_count survivor 1
jq -se '.[0].devices!=.[1].devices' "$out/survivor-device-1.json" "$out/after-plugin-device-1.json" >/dev/null || fail 'Device allocation overlapped after plugin restart.'
snapshot plugin-recovered
delete_workload after-plugin

# Explicit recovery target only: validate exact Minikube worker identity and the
# installed systemd service before restarting kubelet on that worker alone.
topology
platform_guard
no_other_consumers
jq -e --arg node "$GPU_NODE" --arg profile "$PROFILE" '.Nodes|any(.[]; (.Name==$node or ($profile+"-"+.Name)==$node) and .ControlPlane==false and .Worker==true)' "$HOME/.minikube/profiles/$PROFILE/config.json" >/dev/null || fail 'GPU node is not a configured Minikube worker.'
[[ "$(minikube --profile "$PROFILE" ssh --node "$GPU_NODE" -- hostname | tr -d '\r\n')" == "$GPU_NODE" ]] || fail 'SSH target identity mismatch.'
# Minikube's native SSH uses CRLF output even for noninteractive commands.
minikube --profile "$PROFILE" ssh --node "$GPU_NODE" -- 'sudo systemctl show kubelet --property=LoadState --property=ActiveState --property=InvocationID' | tr -d '\r' > "$out/kubelet-before.txt"
grep -qx 'LoadState=loaded' "$out/kubelet-before.txt" || fail 'No loaded kubelet systemd unit.'
grep -qx 'ActiveState=active' "$out/kubelet-before.txt" || fail 'Kubelet is not active.'
grep -Eq '^InvocationID=[0-9a-f]{32}$' "$out/kubelet-before.txt" || fail 'Kubelet invocation identity is absent.'
minikube --profile "$PROFILE" ssh --node "$GPU_NODE" -- sudo systemctl restart kubelet
kubelet_changed() {
  minikube --profile "$PROFILE" ssh --node "$GPU_NODE" -- 'sudo systemctl show kubelet --property=LoadState --property=ActiveState --property=InvocationID' | tr -d '\r' > "$out/kubelet-after.txt" || return 1
  grep -qx 'ActiveState=active' "$out/kubelet-after.txt" && grep -Eq '^InvocationID=[0-9a-f]{32}$' "$out/kubelet-after.txt" && ! diff <(grep '^InvocationID=' "$out/kubelet-before.txt") <(grep '^InvocationID=' "$out/kubelet-after.txt") >/dev/null
}
wait_for kubelet_changed
k wait --for=condition=Ready node/"$GPU_NODE" --timeout=180s
wait_for running survivor 1
device_count survivor 1
create_workload after-kubelet 1
wait_for running after-kubelet 1
device_count after-kubelet 1
# The existing allocation and new allocation must remain exclusive after restart.
jq -se '.[0].devices!=.[1].devices' "$out/survivor-device-1.json" "$out/after-kubelet-device-1.json" >/dev/null || fail 'Device allocation overlapped after kubelet restart.'
snapshot kubelet-recovered
delete_workload survivor
delete_workload after-kubelet
create_workload final-reuse 2
wait_for running final-reuse 2
delete_workload final-reuse
k -n ai-system rollout status daemonset/fake-gpu-device-plugin --timeout=180s
k get nodes -o json | jq -e --arg gpu "$GPU_NODE" --arg r "$RESOURCE" 'all(.items[]; if .metadata.name==$gpu then .status.capacity[$r]=="2" and .status.allocatable[$r]=="2" else .status.capacity[$r]==null and .status.allocatable[$r]==null end)' >/dev/null
echo 'PASS: operator, plugin and real GPU-worker kubelet restart with allocation/reuse.'
