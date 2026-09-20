#!/usr/bin/env bash
# Verify reporting cannot mark a bad response as passed or expose extra fields.
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/scripts/test-report.sh"
DIAGNOSTICS=$(mktemp -d)
trap 'rm -rf -- "$DIAGNOSTICS"' EXIT
LEVEL=5 STAGE=scaling-and-validation TEST_NS=report-test
BODY="$DIAGNOSTICS/body.json"
GITHUB_STEP_SUMMARY="$DIAGNOSTICS/summary.md"

printf '%s\n' '{"namespace":"report-test","name":"demo","replicas":3,"accepted":true,"private_key":"DO_NOT_LOG_KEY","error":"DO_NOT_LOG_ERROR","headers":{"Authorization":"DO_NOT_LOG_TOKEN"}}' >"$BODY"
check_json 'gRPC SetReplicas' 'Persisted count verified' '.replicas == 3 and .accepted == true' >"$DIAGNOSTICS/stdout.log" 2>"$DIAGNOSTICS/stderr.log"
[[ ! -s "$DIAGNOSTICS/stdout.log" ]]
grep -q '^PASS .*gRPC SetReplicas.*"replicas":3' "$DIAGNOSTICS/stderr.log"
if grep -q 'DO_NOT_LOG' "$DIAGNOSTICS/api-checks.log"; then echo 'Sensitive response field was logged' >&2; exit 1; fi

if check_json 'wrong count' 'Expected replicas=4' '.replicas == 4' 2>/dev/null; then
  echo 'Bad response was accepted' >&2; exit 1
fi
grep -q '^FAIL .*wrong count' "$DIAGNOSTICS/api-checks.log"
if grep -q '^PASS .*wrong count' "$DIAGNOSTICS/api-checks.log"; then exit 1; fi

printf '%s\n' 'not JSON' >"$BODY"
if check_json 'malformed response' 'Expected JSON' '.replicas == 3' 2>/dev/null; then
  echo 'Malformed response was accepted' >&2; exit 1
fi
grep -q '^FAIL .*malformed response' "$DIAGNOSTICS/api-checks.log"

printf '%s\n' '{"deployments":[{"namespace":"report-test","name":"demo","replicas":3},{"namespace":"another-tenant","name":"DO_NOT_LOG_WORKLOAD","replicas":1}]}' >"$BODY"
check_json 'ListDeployments' 'Target found' 'any(.deployments[]; .namespace == $ns and .name == "demo")' --arg ns "$TEST_NS" 2>/dev/null
if grep -q 'DO_NOT_LOG' "$DIAGNOSTICS/api-checks.log"; then echo 'Unrelated workload was logged' >&2; exit 1; fi
write_check_summary 1 2>/dev/null
grep -q 'FAILED; passed checkpoints=2 failed checkpoints=2 exit=1' "$DIAGNOSTICS/check-summary.log"
grep -q '### Level 5.*FAILED' "$GITHUB_STEP_SUMMARY"
grep -q 'wrong count' "$GITHUB_STEP_SUMMARY"

# A distinct successful run must have its own summary; append both in one job.
: >"$DIAGNOSTICS/api-checks.log"
STAGE=complete UPGRADE_TEST=1
check_pass 'Expected rejection' 'gRPC InvalidArgument verified' 2>/dev/null
write_check_summary 0 2>/dev/null
grep -q 'PASSED; passed checkpoints=1 failed checkpoints=0 exit=0' "$DIAGNOSTICS/check-summary.log"
[[ $(grep -c '^### Level 5' "$GITHUB_STEP_SUMMARY") == 2 ]]
grep -q 'integration-and-rollout: PASSED' "$GITHUB_STEP_SUMMARY"

STAGE=setup
write_check_summary 0 2>/dev/null
grep -q 'FAILED' "$DIAGNOSTICS/check-summary.log"
unset GITHUB_STEP_SUMMARY
STAGE=complete
write_check_summary 0 2>/dev/null
echo 'Integration report assertions, redaction, output separation and summaries passed.'
