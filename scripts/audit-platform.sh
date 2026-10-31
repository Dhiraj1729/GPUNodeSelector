#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
topology
out="$ROOT/work/day-10/audit"
mkdir -p "$out"
bash "$ROOT/scripts/images.sh" verify
for component in operator mock-inference fake-device-plugin; do
  docker --context "$DOCKER_CONTEXT" image inspect "gpu-node-selector/$component:dev" --format '{{json .}}' | jq '{Id,Architecture,Os,User:.Config.User}' > "$out/$component-image.json"
  user='65532:65532'; [[ "$component" != fake-device-plugin ]] || user='0:0'
  jq -e --arg user "$user" '.User==$user and .Os=="linux"' "$out/$component-image.json" >/dev/null || fail 'Unexpected image user/OS.'
done
k -n ai-system get deployment aiworkload-operator -o json > "$out/operator.json"
jq -e '.spec.replicas==1 and .spec.strategy.type=="Recreate" and .spec.template.spec.serviceAccountName=="aiworkload-operator" and .spec.template.spec.securityContext.runAsNonRoot==true and all(.spec.template.spec.containers[]; .securityContext.readOnlyRootFilesystem==true and .securityContext.allowPrivilegeEscalation==false and (.securityContext.capabilities.drop|index("ALL"))!=null and .readinessProbe.httpGet.path=="/readyz" and .livenessProbe.httpGet.path=="/healthz")' "$out/operator.json" >/dev/null
k -n ai-system get daemonset fake-gpu-device-plugin -o json > "$out/plugin.json"
jq -e '.spec.template.spec.automountServiceAccountToken==false and all(.spec.template.spec.containers[]; .securityContext.privileged!=true and .securityContext.allowPrivilegeEscalation==false and .securityContext.readOnlyRootFilesystem==true and (.securityContext.capabilities.drop|index("ALL"))!=null)' "$out/plugin.json" >/dev/null
k get clusterrole aiworkload-manager-role -o json > "$out/role.json"
jq -e 'all(.rules[]; all(.resources[]; .=="aiworkloads" or .=="aiworkloads/status" or .=="aiworkloads/finalizers" or .=="deployments" or .=="services" or .=="pods" or .=="replicasets") and all(.verbs[]; .!="*" and .!="delete" and .!="deletecollection"))' "$out/role.json" >/dev/null
for resource in nodes nodes/status pods/binding secrets; do
  answer="$(k auth can-i create "$resource" --as=system:serviceaccount:ai-system:aiworkload-operator 2>"$out/auth-warning.txt" || true)"
  [[ "$answer" == no ]] || fail "Operator unexpectedly allowed create $resource, or authorization check failed."
  answer="$(k auth can-i patch "$resource" --as=system:serviceaccount:ai-system:aiworkload-operator 2>>"$out/auth-warning.txt" || true)"
  [[ "$answer" == no ]] || fail "Operator unexpectedly allowed patch $resource, or authorization check failed."
done
k -n ai-system rollout status deployment/aiworkload-operator --timeout=180s
k -n ai-system rollout status daemonset/fake-gpu-device-plugin --timeout=180s
echo 'PASS: image users/architectures/digests, platform security and restricted operator RBAC.'
