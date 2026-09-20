# Integration repairs and validation

The level 2–5 CI logs supplied on 2026-09-20 showed successful cluster creation,
deployment, and initial authenticated reads, followed by a port-forward exit
around an intentional TLS rejection. Subsequent normal requests failed with
connection refused. The two supplied level-5 logs were duplicates.

## Repairs

- `scripts/port-forward.sh` owns one subprocess per stage, checks startup and
  process health, and reaps it before replacement. Each bad identity gets its
  own tunnel and an authenticated reachability check first.
- `cmd/tlscheck` loads certificate fixtures and accepts only a remote TLS
  certificate alert. Missing files, connection refusal, EOF, reset, timeout,
  local server-verification failure, or an accepted connection fail the check.
  This is shared TLS transport validation for both HTTP and gRPC levels.
- A fresh tunnel proves normal authenticated access after both rejections.
  Level 1 also checks access again before reporting success.
- `cmd/probe` announces readiness only after a successful authenticated Service
  request. The harness runs Helm, invokes `/probe --finish` inside the probe Pod,
  and requires ten more seconds of sampling. Its loopback-only control endpoint
  is not exposed by a Service. No additional container package is needed.
- The probe deadline is 300 seconds, the Job deadline 360 seconds, and Helm's
  timeout 180 seconds. Missing completion acknowledgment or an incomplete
  post-upgrade observation period fails closed. A prematurely exited probe
  cannot acknowledge completion.
- Named stages, tunnel/TLS logs, workload logs and events are retained in
  `artifacts/integration/level-N/<test-namespace>/`. CI uploads only selected
  `.log` files for seven days and always removes its own disposable cluster.
  Certificates, private keys and Secret manifests are not artifact inputs.
- `integration-all` now attempts every level and reports all failed levels.
  The previous Bash `-e` configuration already stopped on the first failure;
  it was not silently accepting earlier failures as initially suspected.

The probe samples fresh connections every 200 ms with a three-second request
deadline. gRPC uses `WaitForReady` within that deadline. Passing demonstrates
requests completing within that bound during the observed upgrade; it does not
prove zero latency or detect every interruption between samples.
The HTTP probe checks response status and attempts to drain the body; it does
not currently fail on a body-drain error or validate the returned JSON payload.
The recorded request counts therefore are not complete response-integrity tests.

## Repeat locally

Run from the repository root in Linux/WSL; paths derive from the checkout:

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/dev.sh" quality vuln docker-test
bash "$repo_root/scripts/dev.sh" integration-all CLUSTER=sre-ci-fix
for level in 3 4 5; do
  bash "$repo_root/scripts/dev.sh" upgrade-test "LEVEL=$level" CLUSTER=sre-ci-fix || exit 1
done
bash "$repo_root/scripts/dev.sh" clean-cluster CLUSTER=sre-ci-fix
```

`quality` includes the real TLS regression tests, rollout-monitor tests and shell
harness tests. Those deliberately exercise a closed listener, canceled request,
missing fixture, rejected identities, dead forwarding subprocess, missing finish
signal and a post-upgrade outage. They require no Kubernetes cluster.

## Execution evidence

Validated on 2026-09-20 in Ubuntu-24.04 under WSL, Linux/ARM64, using Go 1.27.1,
Docker 29.7.2, KIND 0.33.0, Kubernetes 1.37.0 and Helm 4.3.0. The dedicated
`sre-ci-fix` cluster exercised levels sequentially; CI gives each matrix level
its own Ubuntu runner and cluster. The source baseline was commit
`3e4603a85f0ccb9aa9a956167a6d3eb11e558ee4` plus the local working-tree changes.
The [machine-readable receipt](validation/integration-summary.json) records the
tested code hashes and results.

| Check | Result |
|---|---|
| Go formatting | PASS, all 29 tracked/nonignored Go files, including generated code |
| Quality | PASS: generated-source comparison, race tests, vet, build, chart rendering, workflow validation, shell harness, module verification |
| Vulnerability scan | PASS, no vulnerabilities found |
| Docker test stage and runtime image | PASS on Linux/ARM64 |
| Real KIND integration | PASS for levels 1, 2, 3, 4 and 5 |
| Deliberately stopped real kubectl tunnel | Correctly failed TLS rejection check with connection refused |
| Missing certificate fixture | Correctly failed before attempting TLS |
| Offline visual guides | PASS, six pages, links/layout/playback/GIF motion and reduced-motion defaults |

| Rollout level | Authenticated requests | Failures | Longest request |
|---|---:|---:|---:|
| 3 | 144 | 0 | 106.25 ms |
| 4 | 146 | 0 | 114.24 ms |
| 5 | 137 | 0 | 86.96 ms |

Each rollout passed the completion acknowledgment, ten-second post-upgrade
observation, final authenticated read and scale-to-zero checks. Level 5 also
passed drift correction, intent readiness and HPA conflict checks.

The first aggregate attempt failed level 1 because the script was edited while
that process was reading it. Its unchanged rerun passed. A local Docker/KIND
restart interrupted the next validation attempt; restarting only the named test
node and keeping the remaining checks in one WSL session resolved it. Those
earlier attempts are preserved separately from the passing results.

Detailed local outputs remain in ignored `artifacts/validation/` and
`artifacts/integration/`; the receipt above is suitable for committing. GitHub's
hosted Linux/AMD64 jobs have not yet run these working-tree changes. Push the
reviewed changes to obtain that separate CI evidence. These tests do not close
every interview acceptance gap in the historical audit, such as an explicit
leader-failover exercise or a Kubernetes API connectivity-loss demonstration.
