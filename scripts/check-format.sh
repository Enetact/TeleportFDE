#!/usr/bin/env bash
set -euo pipefail
bad=$(gofmt -l cmd internal)
if [[ -n "$bad" ]]; then printf 'Unformatted Go files:\n%s\n' "$bad" >&2; exit 1; fi
