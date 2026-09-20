# Read the API test evidence

**Author and system architect:** Jamie Holland.

A successful integration step means the harness completed its assertions and
exited with code zero. Invalid input, stale versions, HPA conflicts and
unauthorized clients are expected to be rejected: those tests pass when the
expected rejection is observed. A skipped integration job provides no
integration evidence, even if the policy or source-quality job is green.

Helm may print `TEST SUITE: None` during installation because the chart has no
Helm test hooks. Our checks run afterward through `scripts/integration.sh`;
the Helm install message is not the integration test result.

## Find results in GitHub

1. Open **Actions → development-ci → the run → integration (level)**.
2. Expand **Verify level N API calls** for named stages and explicit `PASS`/`FAIL`
   checkpoints showing the operation and observed result.
3. For levels 3–5, also open **Probe rolling availability** for the separate
   integration/rollout run and request statistics.
4. On the run summary, inspect the per-level **PASSED/FAILED** section and expand
   **API checks and observed results**. Integration and rollout steps produce
   separate summaries.
5. Download `integration-level-N-<attempt>` for the retained `.log` evidence.
   Retention is seven days, including failed runs when upload can execute.

Reports use GitHub's [job summary support](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands#adding-a-job-summary).
They are regular output; no Actions debug setting is needed. Hosted reports
appear after the changes are pushed and the event enables full checks; see
[CI controls](CI-CONTROLS.md). Setup failures or forced termination before the
harness executes may leave no report: inspect the failed/canceled step itself.

## What a PASS line establishes

| Checkpoint | Evidence required before PASS is printed |
|---|---|
| Replica read | Request succeeds; JSON identifies the fixture and has the expected count, except the final identity-only read |
| Replica write | Request succeeds; returned namespace/name/count match; level 5 also requires `accepted: true` |
| List | Request succeeds; the fixture is in the returned Deployment list |
| Kubernetes count | Live Deployment spec reaches the expected count within the polling budget |
| Invalid count | HTTP 400 or a failed CLI call reporting gRPC `InvalidArgument` |
| Stale version | HTTP 409 or a failed CLI call reporting gRPC `Aborted` |
| HPA conflict | HTTP 412 or a failed CLI call reporting gRPC `FailedPrecondition` |
| Missing/unauthorized identity | The TLS verifier confirms the intended remote certificate rejection |
| Drift correction | After an external scale to 1, the Deployment and API return to intent count 3 |
| Intent readiness | Deployment rollout succeeds; intent phase is Ready and its observed generation matches |
| Rollout probe | Job completes after the finish acknowledgment and post-upgrade sampling |

Successful read/write lines include selected JSON response fields. Error tests
print the verified status rather than complete error bodies. Shortened format
examples, not a new execution receipt:

```text
PASS level=2 stage=scaling-and-validation | HTTP PUT ... replicas=3 | Write response verified ... response={"namespace":"sre-it-example","name":"demo","replicas":3}
PASS level=2 stage=scaling-and-validation | Kubernetes GET Deployment sre-it-example/demo | spec.replicas=3 observed after 1 attempt(s)
PASS level=2 stage=scaling-and-validation | HTTP PUT ... replicas=-1 | Expected HTTP 400 rejection verified
PASS level=5 stage=scaling-and-validation | Controller drift correction | Manual spec.replicas=1 was restored to persisted desired count 3
```

A failed response assertion prints `FAIL`; a failed/interrupted harness records
a failed run checkpoint. The process returns nonzero. Earlier PASS lines do not
override a later failure.

## Logs and boundaries

Each invocation has its own directory:

```text
artifacts/integration/level-N/<test-namespace>/
  api-checks.log       Verified operations, selected responses and failures
  check-summary.log    Overall outcome, checkpoint counts and exit code
  stages.log          Stage boundaries
  result.log          Final stage and exit status
  tls-*.log           Certificate-rejection verifier output
  port-forward-*.log  Tunnel lifecycle output
  rollout.log         Rollout statistics when an upgrade is tested
```

Failures also retain Kubernetes workload/event diagnostics. The directory is
ignored by Git and covered by the workflow's existing `.log` artifact rule.

Polling is summarized by its final verified count and attempt number. Rollout
traffic retains aggregate request/failure/timing statistics. Checkpoint counts
are not total API request counts or coverage measurements. Reporting adds no
new API requests and does not enable packet/body dumps.

The API report shows selected names, counts, versions and intent fields from
the disposable fixture. It excludes headers, private keys, arbitrary error-body
fields and unrelated Deployment list entries. Existing diagnostic logs have
their own scope; this is not a production audit trail or a general-purpose
redaction system.

`accepted: true` means saved intent, not ready Pods. Cached responses can lag
writes. Rollout results retain the sampling and response-integrity limits in
[integration validation](INTEGRATION-VALIDATION.md).

HTTP direct-write responses show `accepted: false`: that field marks the deferred
level-5 intent path, not whether an HTTP write failed. For levels 2–4, the
successful request and verified target/count establish the scale write; separate
Kubernetes/API observations establish convergence.

## Local reproduction

From the repository root in Linux/WSL:

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/dev.sh" harness-check workflow-check
bash "$repo_root/scripts/dev.sh" integration LEVEL=2 CLUSTER=sre-demo
```

The same output appears locally. To also create a Markdown summary:

```bash
mkdir -p "$repo_root/artifacts/terminal-demo"
export GITHUB_STEP_SUMMARY="$repo_root/artifacts/terminal-demo/api-summary.md"
bash "$repo_root/scripts/dev.sh" integration LEVEL=5 CLUSTER=sre-demo
unset GITHUB_STEP_SUMMARY
```

The summary appends; select a new filename for a separate demonstration.
[test-report_test.sh](../scripts/test-report_test.sh) covers bad counts, malformed
JSON, excluded fields, stdout/stderr separation and success/failure summaries.
It runs in `harness-check` and therefore in the CI quality gate.

[integration.sh](../scripts/integration.sh) owns assertions and stage progression;
[test-report.sh](../scripts/test-report.sh) writes the reports. The
[terminal demo](TERMINAL-DEMO.md) provides the interactive API walkthrough.

## Verification of this reporting change

On 2026-09-20, local Ubuntu WSL/Linux ARM64 validation passed:

- Reporting regression tests, harness lifecycle checks, CI policy tests and actionlint.
- Level 4 integration plus rollout: 21 passed checkpoints, zero failed checkpoints;
  the rollout sampled 152 requests with zero failures (longest 203.917173 ms).
- Level 5 integration on a fresh cluster: 23 passed checkpoints, zero failed
  checkpoints, including gRPC error checks, drift correction and intent readiness.

An earlier level-5 attempt on the sequentially reused cluster timed out waiting
for the disposable Deployment during setup. It correctly produced a FAILED
summary with zero passed checkpoints. Its logs remain separate from the fresh
cluster's passing run; neither assertions nor timeouts were relaxed for the retry.
Both dedicated test clusters were deleted.

The [source-hash receipt](validation/api-reporting-summary.json) records results
and local log locations. Levels 1–3 were not rerun for this logging change, and
GitHub-hosted execution remains separate evidence.
