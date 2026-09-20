#!/usr/bin/env bash
# Sourced by the integration harness. One tunnel belongs to one test stage;
# a deliberate TLS rejection may terminate kubectl's entire forwarding session.
stop_forward() {
  if [[ -n "${PF_PID:-}" ]]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
    PF_PID=""
  fi
}

forward_alive() {
  [[ -n "${PF_PID:-}" ]] && kill -0 "$PF_PID" 2>/dev/null || {
    echo "Port-forward exited during stage: ${STAGE:-unknown}" >&2
    return 1
  }
}

start_forward() {
  stop_forward
  PF_LOG="$DIAGNOSTICS/port-forward-$1.log"
  : >"$PF_LOG"
  "${K[@]}" -n "$NAMESPACE" port-forward --address=127.0.0.1 "service/$RELEASE" "$PORT:8443" >"$PF_LOG" 2>&1 &
  PF_PID=$!
  for _ in {1..100}; do
    forward_alive || return 1
    # Reading readiness output avoids opening an unauthenticated TCP connection.
    if grep -q 'Forwarding from 127.0.0.1:' "$PF_LOG"; then return 0; fi
    sleep 0.1
  done
  echo "Port-forward did not become ready: $PF_LOG" >&2
  return 1
}
