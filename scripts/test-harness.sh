#!/usr/bin/env bash
# Exercise process ownership and fail-closed behavior without a Kubernetes cluster.
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/scripts/port-forward.sh"
DIAGNOSTICS=$(mktemp -d)
PF_PID=""
trap 'stop_forward; rm -rf -- "$DIAGNOSTICS"' EXIT
STAGE=regression
NAMESPACE=test RELEASE=test PORT=18443
K=(fake_kubectl)
fake_kubectl() {
  printf 'Forwarding from 127.0.0.1:18443 -> 8443\n'
  exec sleep 60
}
start_forward healthy
forward_alive
old_pid=$PF_PID
start_forward replacement
if kill -0 "$old_pid" 2>/dev/null; then echo 'Previous tunnel leaked' >&2; exit 1; fi
kill "$PF_PID"
wait "$PF_PID" 2>/dev/null || true
if forward_alive; then echo 'Dead tunnel accepted' >&2; exit 1; fi
stop_forward
fake_kubectl() { echo 'unable to connect' >&2; return 1; }
if start_forward unavailable; then echo 'Failed startup accepted' >&2; exit 1; fi
stop_forward
cat >"$DIAGNOSTICS/fake-make" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$2" >>"$HARNESS_LEVELS"
[[ "$2" != LEVEL=2 ]]
SH
chmod +x "$DIAGNOSTICS/fake-make"
export HARNESS_LEVELS="$DIAGNOSTICS/levels.log"
if make -s -C "$repo_root" integration-all MAKE="$DIAGNOSTICS/fake-make" >"$DIAGNOSTICS/aggregate.log" 2>&1; then
  echo 'Aggregate accepted a failed level' >&2; exit 1
fi
[[ $(cat "$HARNESS_LEVELS") == $'LEVEL=1\nLEVEL=2\nLEVEL=3\nLEVEL=4\nLEVEL=5' ]]
grep -q 'Failed integration levels: 2' "$DIAGNOSTICS/aggregate.log"
while IFS= read -r -d '' script; do bash -n "$script"; done < <(find "$repo_root/scripts" "$repo_root/levels" -name '*.sh' -print0)
echo 'Harness lifecycle and shell syntax checks passed.'
