#!/usr/bin/env bash
# Source this file to select the repository's installed development tools.
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/toolchain.env"
tool_home="${TELEPORT_TOOL_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}/teleportfde}"
export PATH="$tool_home/go-$GO_VERSION/bin:$tool_home/bin:$repo_root/.tools/bin:$PATH"
export GOTOOLCHAIN=local
