#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
export PATH="/opt/homebrew/bin:/opt/homebrew/sbin:$PATH"
export HOMEBREW_NO_AUTO_UPDATE=1
export HOMEBREW_NO_INSTALL_UPGRADE=1
export HOMEBREW_NO_INSTALL_CLEANUP=1
if ! xcode-select -p >/dev/null 2>&1; then
  echo 'ERROR: install Apple Command Line Tools with xcode-select --install first.' >&2
  exit 1
fi
if ! command -v brew >/dev/null 2>&1; then
  echo 'ERROR: Homebrew is required. Install it from https://brew.sh, then rerun make tools.' >&2
  exit 1
fi
# Preserve existing working commands, including system Git/jq and user installations.
# Keep this mapping aligned with Brewfile; no brew upgrade or bulk bundle operation.
for pair in go:go git:git colima:colima docker:docker minikube:minikube kubectl:kubernetes-cli kubebuilder:kubebuilder jq:jq rg:ripgrep; do
  tool="${pair%%:*}"
  formula="${pair#*:}"
  if command -v "$tool" >/dev/null 2>&1; then
    printf 'KEEP %s: %s\n' "$tool" "$(command -v "$tool")"
  else
    printf 'INSTALL missing command %s from %s\n' "$tool" "$formula"
    brew install "$formula"
  fi
done
if ! docker buildx version >/dev/null 2>&1; then
  if ! brew list --versions docker-buildx | /usr/bin/grep -q .; then
    brew install docker-buildx
  fi
  # Installing the formula may repair an existing dangling symlink.
  if docker buildx version >/dev/null 2>&1; then
    /bin/bash "$project_root/scripts/doctor.sh"
    exit 0
  fi
  plugin_dir="${DOCKER_CONFIG:-$HOME/.docker}/cli-plugins"
  plugin_path="$plugin_dir/docker-buildx"
  plugin_source="$(brew --prefix docker-buildx)/bin/docker-buildx"
  if [[ ! -x "$plugin_source" ]]; then
    echo "ERROR: Buildx binary unavailable at $plugin_source" >&2
    exit 1
  fi
  if [[ -e "$plugin_path" || -L "$plugin_path" ]]; then
    echo "ERROR: existing Buildx entry does not work; inspect $plugin_path (not overwritten)." >&2
    exit 1
  fi
  mkdir -p "$plugin_dir"
  ln -s "$plugin_source" "$plugin_path"
fi
/bin/bash "$project_root/scripts/doctor.sh"
