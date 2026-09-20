#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
if [[ $# -eq 0 ]]; then set -- help; fi
exec bash "$repo_root/scripts/dev.sh" "$@" LEVEL=5
