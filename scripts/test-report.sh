#!/usr/bin/env bash
# Sourced by the integration harness. Report verified checkpoints, not shell traces.
# Write to stderr so capturing an API's stdout never captures report text.
record_check() {
  local result=$1 operation=$2 detail=$3
  printf '%s level=%s stage=%s | %s | %s\n' \
    "$result" "$LEVEL" "$STAGE" "$operation" "$detail" \
    | tee -a "$DIAGNOSTICS/api-checks.log" >&2
}

check_pass() { record_check PASS "$1" "$2"; }
check_fail() { record_check FAIL "$1" "$2"; }

check_json() {
  local operation=$1 detail=$2 predicate=$3 summary
  shift 3
  if ! jq -e "$@" "$predicate" "$BODY" >/dev/null; then
    check_fail "$operation" "Response assertion failed: $detail"
    return 1
  fi
  # Only show the fixture's fields; never dump headers, credentials, error text,
  # or the cluster-wide list. JSON encoding keeps embedded newlines on one line.
  if ! summary=$(jq -c --arg ns "$TEST_NS" '
    if has("deployments") then
      {matchingDeployments: [.deployments[] |
        select(.namespace == $ns and .name == "demo") | {namespace, name, replicas}]}
    else
      {namespace, name, replicas, readyReplicas, availableReplicas,
       desiredReplicas, accepted, version, intentVersion, reconcilePhase} |
      with_entries(select(.value != null))
    end' "$BODY"); then
    check_fail "$operation" 'Could not summarize the validated response'
    return 1
  fi
  check_pass "$operation" "$detail; response=$summary"
}

write_check_summary() {
  local code=$1 passed=0 failed=0 outcome=FAILED mode=integration
  [[ ${UPGRADE_TEST:-0} == 1 ]] && mode=integration-and-rollout
  [[ $code == 0 && $STAGE == complete ]] && outcome=PASSED
  if [[ -f "$DIAGNOSTICS/api-checks.log" ]]; then
    passed=$(grep -c '^PASS ' "$DIAGNOSTICS/api-checks.log" || true)
    failed=$(grep -c '^FAIL ' "$DIAGNOSTICS/api-checks.log" || true)
  fi
  printf 'Level %s %s: %s; passed checkpoints=%s failed checkpoints=%s exit=%s\n' \
    "$LEVEL" "$mode" "$outcome" "$passed" "$failed" "$code" \
    | tee "$DIAGNOSTICS/check-summary.log" >&2 || return 1
  if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then
    {
      printf '### Level %s — %s: %s\n\n' "$LEVEL" "$mode" "$outcome"
      printf '| Passed checkpoints | Failed checkpoints | Exit code | Last stage |\n'
      printf '|---:|---:|---:|---|\n| %s | %s | %s | %s |\n\n' "$passed" "$failed" "$code" "$STAGE"
      printf 'Counts describe verified checkpoints, not individual poll/rollout requests.\n\n'
      printf '<details><summary>API checks and observed results</summary>\n\n```text\n'
      if [[ -f "$DIAGNOSTICS/api-checks.log" ]]; then cat "$DIAGNOSTICS/api-checks.log"; fi
      printf '```\n</details>\n\n'
    } >>"$GITHUB_STEP_SUMMARY"
  fi
}
