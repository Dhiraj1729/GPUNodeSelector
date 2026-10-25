#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib/environment.sh"
topology
k label node "$GPU_NODE" ai.dhiraj.io/gpu-node=true ai.dhiraj.io/gpu-product=A100 --overwrite
k label node "$CPU_NODE" ai.dhiraj.io/cpu-node=true --overwrite
k taint node "$GPU_NODE" ai.dhiraj.io/gpu=true:NoSchedule --overwrite
k get node "$GPU_NODE" -o json | jq -e '.metadata.labels["ai.dhiraj.io/gpu-product"]=="A100" and any(.spec.taints[]; .key=="ai.dhiraj.io/gpu" and .value=="true" and .effect=="NoSchedule")'
