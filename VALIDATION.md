# Current validation status

The latest local validation passed on 2026-09-20 using Ubuntu-24.04 under WSL,
Linux/ARM64, Go 1.27.1 and Kubernetes 1.37.0.

| Scope | Current evidence |
|---|---|
| Formatting, full race tests, vet, binaries, charts and workflow syntax | Passed; 29 Go files and four host binaries |
| Go vulnerability scan and Docker test/build | Passed; no Go vulnerabilities found |
| Real KIND integration | Levels 1–5 passed |
| Service rollout monitoring | Levels 3–5 passed; 427 sampled requests, zero failures |
| Broken tunnel and missing certificate | Correctly failed the negative-test verifier |
| Offline visual guides | All six pages passed browser validation |
| GitHub-hosted execution of the new fixes | Not yet recorded; local success is separate evidence |

Use the [integration report](docs/INTEGRATION-VALIDATION.md) and
[source-hash receipt](docs/validation/integration-summary.json) for the exact
scope, earlier attempts, timings and environment. The temporary KIND cluster was
removed. [Development setup](docs/DEVELOPMENT.md) contains the current commands;
[dependency validation](docs/DEPENDENCY-VALIDATION.md) records the earlier package
and image work. Explicit leader-failover, connectivity-loss, certificate-rotation
and load testing remain separate from the checks recorded here.

The imported review below is retained for provenance. Its review-specific receipt
directory is not present in this checkout, and its proposed behavioral corrections
are not automatically part of this implementation. Its old tool versions, missing
build-input claims and commands are historical, not current setup instructions.

## Imported review record (historical)

**Historical review dated September 20, 2026; superseded for current status by the records above.**

Status: source-reviewed, dependency-free core executed. Full application, gRPC/Kubernetes adapters and deployment **not verified**. Proposed Level 6 features are design only.

## Environment and reproducibility boundary

Installed Go: `go version go1.23.2 linux/amd64`. The full module requires Go >=1.25.0. The existing builder/CI selects Go 1.26.8. Module downloads were unavailable in this runtime; protoc, Helm, Docker and a live Kubernetes test environment were unavailable. The separate `go.offline.mod` permits only the explicitly enumerated dependency-free package test set; it is not a replacement for the full module.

## Executed checks

| Check | Result | Evidence file under `validation/review-2026-09-20/` |
|---|---|---|
| Original core race-enabled tests | Pass: 76 test/subtest events, 13 top-level, six tested packages. | `baseline-tests.jsonl`, `baseline-tests.stderr` |
| Reviewed core race-enabled tests | Pass: 107 test/subtest events, 21 top-level including two fuzz-seed parents, six tested packages. | `reviewed-tests.jsonl`, `reviewed-tests.stderr` |
| Reviewed core vet | Pass, no diagnostics. | `reviewed-vet.txt` |
| Original documentation-presence scan | 66 missing-documentation entries. | `baseline-docs.json` |
| Revised documentation-presence scan | Zero findings; 14 non-test Go files. | `reviewed-docs.json` |
| Selected new regressions applied to original | Expected failure: unknown codes, internal-message exposure, pre-canceled service/reconcile work. | `regression-before.txt` |
| ParseKey fuzz | Pass; 133,450 executions. | `fuzz-model.txt` |
| Mutation JSON fuzz | Pass; 48,222 executions. | `fuzz-http.txt` |
| Full-module build attempt | Blocked: local Go version lower than module minimum; compilation did not begin. | `full-build-attempt.txt` |
| Generated-output prerequisite | Expected failure: actual go.sum/protobuf bindings absent. | `generated-gate.txt` |
| Final formatting/shell syntax/core rerun | See result log and exit status. | `final-checks.txt` |

Fuzzing used a 3-second budget per target and two workers. Actual execution time includes initialization and shutdown. Counts are observations, not exhaustive input-space coverage. Active fuzz runs did not enable `-race`; the normal core suite did. `cmd/certgen` compiled with no test functions.

## Core commands actually used

```bash
GOTOOLCHAIN=local GOPROXY=off go test -modfile=go.offline.mod -race -count=1 -json \
  ./internal/model ./internal/service ./internal/httpapi ./internal/reconcile \
  ./internal/security ./internal/health ./cmd/certgen

GOTOOLCHAIN=local GOPROXY=off go vet -modfile=go.offline.mod \
  ./internal/model ./internal/service ./internal/httpapi ./internal/reconcile \
  ./internal/security ./internal/health ./cmd/certgen

GOTOOLCHAIN=local GOPROXY=off go test -modfile=go.offline.mod -run '^$' \
  -fuzz '^FuzzParseKey$' -fuzztime=3s -parallel=2 ./internal/model

GOTOOLCHAIN=local GOPROXY=off go test -modfile=go.offline.mod -run '^$' \
  -fuzz '^FuzzDecodeSetRequest$' -fuzztime=3s -parallel=2 ./internal/httpapi

bash scripts/check-format.sh
bash -n scripts/*.sh
```

The documentation scanner uses Go's parser/AST to check comment presence on declarations. Its source is included as `scan_go.go.txt`; copy outside the module as a `.go` file before running it. It excludes test files and is not a replacement for godoc review, go vet, a linter, or type checking.

## Not executed / must pass before release

The complete supported-toolchain build; `go mod tidy`/verify on the full module; protobuf generation/regeneration; gRPC and Kubernetes-dependent tests; server/controller/probe binaries; govulncheck; container image build/scanning; Helm lint/render/admission; KIND integration for all five levels; real-Service rolling-upgrade tests; certificate rotation, leader-loss and crash-recovery testing.

The revised CI and Makefile define gates for several of these, but defining a gate does not prove it passed. The archive intentionally does not fabricate `go.sum` or generated protobuf bindings. Run `make prepare`, review/commit outputs, and then use the full sequence in [GO_REVIEW.md](GO_REVIEW.md).

`quality-dry-run.txt` in the working investigation was not included as passing evidence: recursive make recipes ran during the attempted dry run and failed at absent generation prerequisites. No successful full quality run is claimed.

## Interpretation

The core behavioral corrections have direct execution evidence. The external-adapter corrections have only source-review evidence. The remaining security and deployment risks in [GO_REVIEW.md](GO_REVIEW.md) are open, not waived by passing unit tests. Earlier logs in `docs/validation/` are retained as historical authoring evidence and are superseded by this dated record.
