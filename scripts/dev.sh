#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/dev-env.sh"
if [[ $# == 0 ]]; then set -- help; fi
exec make -C "$repo_root" "$@"
