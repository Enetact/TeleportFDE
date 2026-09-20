#!/usr/bin/env bash
# One file inventory for local formatting and the CI formatting gate.
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/scripts/dev-env.sh"
cd -- "$repo_root"
mode=${1:---check}
[[ $mode == --check || $mode == --write ]] || { echo 'Usage: check-format.sh [--check|--write]' >&2; exit 2; }
manifest="$(mktemp)"
trap 'rm -f -- "$manifest"' EXIT
# Include untracked source during development; honor .gitignore for tools and scratch files.
git ls-files --cached --others --exclude-standard -z -- '*.go' > "$manifest"
files=()
while IFS= read -r -d '' file; do
  [[ -f $file ]] && files+=("$file")
done < "$manifest"
[[ ${#files[@]} -gt 0 ]] || { echo 'No Go source files found.' >&2; exit 1; }
if [[ $mode == --write ]]; then gofmt -w "${files[@]}"; fi
bad=$(gofmt -l "${files[@]}")
if [[ -n $bad ]]; then
  printf 'Unformatted Go files:\n%s\nRun: bash scripts/dev.sh format\n' "$bad" >&2
  exit 1
fi
printf 'Go formatting passed for %s files (including generated source).\n' "${#files[@]}"
