#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
if [[ $# -eq 0 ]]; then set -- help; fi
exec make -C "$repo_root" "$@" LEVEL=1
