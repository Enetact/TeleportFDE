# Validation record

> Historical authoring record. The environment, tool versions and NOT RUN entries below describe the original pass. For current executed results, see [dependency validation](DEPENDENCY-VALIDATION.md) and [integration validation](INTEGRATION-VALIDATION.md).

Prepared September 20, 2026. This record distinguishes executed checks from
provided-but-unexecuted tests. It is not a claim that all five levels run end to end.

## Environment actually used

Linux amd64, Go 1.23.2, working local compiler/race instrumentation, and OpenSSL.
No Go module downloads were available. Docker, Kubernetes, Helm, and protoc were
not present. The full application's Go module requires Go 1.25 or newer.

The offline module file only permits the dependency-free packages to be tested
with the available toolchain. It is not the application's dependency graph and
must not be used to build the Kubernetes/gRPC server.

## Executed successfully

| Check | Result |
|---|---|
| Core tests with `-race -count=1` | PASS; 76 test/subtest pass events, including parent test events; 13 top-level tests across six tested packages |
| `go vet` on the six core packages plus certgen | PASS after resolving unkeyed test literals |
| Go formatting for every authored Go file | PASS |
| Bash syntax for the supplied shell scripts | PASS |
| Plain YAML parsing: Chart, values, CRD, workflow | PASS; this is not Helm template rendering or Kubernetes admission validation |
| Helm values JSON schema syntax | PASS; JSON parsed |
| Dependency-free certgen compilation and execution | PASS |
| OpenSSL verification of generated server/client identities with the corresponding EKUs | PASS |
| Make recipe expansion for the integration target | PASS; this does not execute its tools or validate their effects |

Executed package statement coverage:

| Package | Coverage |
|---|---:|
| internal/model | 82.1% |
| internal/service | 60.0% |
| internal/httpapi | 94.6% |
| internal/reconcile | 96.2% |
| internal/security | 77.4% |
| internal/health | 90.0% |

These numbers are coverage from each package's own tests, not whole-project
coverage. `cmd/certgen` had no Go unit tests and reported 0% instrumented coverage;
its binary was separately built and exercised with OpenSSL verification.

The mTLS tests exercised authorized clients plus missing certificates, wrong URI,
wrong CA, wrong server name, expired identity, server-only EKU used as a client,
and TLS 1.2 rejection. Controller-decision tests exercised drift, zero, ready and
not-ready states, changed intent, HPA conflict, protected/deleting targets,
missing targets/intents, UID replacement, and error propagation.

Raw execution records:

- `validation/core-tests.jsonl`
- `validation/core-vet.txt`

## Written but NOT executed here

| Component/check | Status |
|---|---|
| Protobuf Go source generation | NOT RUN; generated with `make prepare` on the developer's machine |
| Real go.sum / complete dependency resolution | NOT RUN; must resolve, review, and commit the real lockfile |
| Full `go test -race ./...` and `go vet ./...` | NOT RUN |
| client-go fake-client tests | WRITTEN, NOT RUN |
| gRPC wire/codec and error-mapping tests | WRITTEN, NOT RUN |
| Server, replicactl and rollout-probe binary compilation | NOT RUN |
| Helm lint and rendering for levels 1–5 | COMMANDS PROVIDED, NOT RUN |
| Kubernetes CRD schema/CEL admission | NOT RUN |
| Docker multi-stage build | NOT RUN |
| Live integration for levels 1–5 | SCRIPT PROVIDED, NOT RUN |
| Level 4/5 availability during Helm upgrade | IN-CLUSTER PROBE PROVIDED, NOT RUN |
| GitHub Actions workflow | PROVIDED, NOT RUN |
| Dependency vulnerability scanning or independent security review | NOT RUN |

## Verification sequence to complete

```bash
make doctor
make prepare
make test
make vet
make build
make helm-check
make integration-all
make upgrade-test LEVEL=4
make upgrade-test LEVEL=5
```

Review and commit the actual `go.mod`, `go.sum`, and generated protobuf files after
bootstrap. CI deliberately checks that they have been committed and remain stable
under regeneration. A CI failure before that first commit is an explicit
bootstrap/lock requirement, not a passing release result.

Only after these checks pass on the target toolchain/cluster can the implementation
be described as build- and integration-verified. Even then, production readiness
requires the additional security and operational reviews described in SECURITY.md.
