# Go source review and correction record

> Historical imported review notes, not a description of the current checkout or its latest test results. Current results are in [dependency validation](docs/DEPENDENCY-VALIDATION.md) and [integration validation](docs/INTEGRATION-VALIDATION.md). The review-specific receipt directory referenced below is absent here, so its fuzz counts and claimed corrections are not verified evidence for this tree. For example, Backend still lives in internal/model, and the HTTP probe still ignores body-drain errors; do not infer those imported fixes from the passing integration tests.

**Reviewed September 20, 2026 | Prepared for Jamie Holland**

## Historical verdict and scope

The revised source is better aligned with Go's published coding guidance, with specific behavioral regressions fixed and tested in the dependency-free core. It is **not yet a fully built, vulnerability-cleared, or deployment-validated release**. The Kubernetes/gRPC portions remain source-reviewed, not type-checked or executed here. No Level 6 feature is implemented in this revision.

The original archive was preserved. Its SHA-256 is `31603215ee134f60a60521579b88440b62e84b53d1e4ef3aa4274248543d1c65`.

The review covered the original archive's 14 non-test Go files, tests, build automation, and relevant deployment/security assumptions. It combined source inspection, a small AST documentation-presence scan, regression tests, race-enabled core tests, short fuzz runs, formatting, and core vet checks. A documentation-presence scan cannot establish the accuracy or completeness of every API contract.

Go has a language specification and multiple complementary guidance documents, not a single production-readiness certification checklist. Effective Go remains useful for idioms, but its own introduction explains its age and limited coverage of later language/ecosystem changes. This review also uses Code Review Comments, Go Doc Comments, the context documentation, module guidance, and official testing/security documentation. [G1–G8]

## Changes claimed by the imported review

| ID | Finding | Change and files | Evidence/status |
|---|---|---|---|
| G-01 | Missing package/export documentation; some import groups mixed standard and external packages. | Added contract comments, including nil-versus-zero, concurrency expectations and persisted-versus-converged semantics. Standard-library imports first; gofmt applied. | Documentation scan: 66 missing-documentation entries before, zero after. Formatting passes. This is a presence scan, not a prose-quality certification. |
| G-02 | Persistence abstraction lived in `model`, although `service` consumes it. | Moved the three-method `Backend` interface to `internal/service`; concrete Kubernetes implementation remains separate. | Core tests and vet pass. Consumer-owned interfaces follow Go review guidance; no new dependency-injection framework added. |
| G-03 | An unknown/future error code could reach a missing gRPC map entry, which has the zero `codes.OK` value. | Normalize unknown model codes to `Internal`; explicitly default unknown gRPC mappings to `codes.Internal`. Preserve nil input as nil. | Model regressions pass. Added gRPC regressions are **not executed** because external dependencies/generated code are unavailable. |
| G-04 | Internal error messages could be exposed through the public error helper. | Redact Internal and unknown-code messages; preserve wrapped causes for internal inspection. | Privacy and `errors.Is` regression tests pass. This is not a comprehensive log-redaction audit. |
| G-05 | A pre-canceled request could still reach a cached/fake backend or reconciliation store. | Check `ctx.Err()` before service Get/List/Set and reconciliation work. | Canceled/expired tests pass and demonstrate zero backend calls. Selected tests fail against the original implementation. Cancellation cannot undo a mutation already committed remotely. |
| G-06 | Standard JSON struct decoding accepts ambiguous duplicate/case-aliased field forms. | Small token-based decoder using `encoding/json` rejects duplicates, aliases, unknown keys, trailing values, and null expectedVersion; keeps the existing request-body limit. | Strict-decoding cases and fuzzing pass. This is deliberate API hardening, **not a Go language requirement**. |
| G-07 | Startup/error paths could leave controller queue or network resources open. | Add explicit controller Shutdown, constructor cleanup, listener/admin-server cleanup; document drain-context cancellation ownership. | Source-reviewed only for controller/server. Full lifecycle, standby, leadership-loss and goroutine-join tests remain required. |
| G-08 | Upgrade probe ignored an HTTP response-body read error. | Check `io.Copy` error before recording success. | Source-reviewed; probe and live upgrade tests remain unexecuted. |
| G-09 | Verification could implicitly bootstrap/regenerate dependency state. | Separate `prepare` from checks. Require real generated bindings/checksums, verify regeneration, use readonly module mode, add fuzz and vulnerability-scan gates. | Missing-output gate fails as intended. Full workflow **not run**. |

### The error-mapping defect, precisely

This was a latent fail-open mapping problem, not evidence that every original error was misreported. The currently enumerated codes were mapped. An unrecognized code could nevertheless fall through a map lookup without its `ok` result, producing zero. gRPC's `status.Error(codes.OK, ...)` returns nil. The revised code checks the lookup and chooses `codes.Internal`; the model layer independently normalizes unknown codes. [G9]

The related privacy fix ensures a detailed internal error cannot become the client-visible message merely because it is wrapped in the custom error type. Diagnostics belong in protected internal records, not in arbitrary remote responses.

### Go design decisions retained

The implementation keeps explicit error returns, small packages, concrete structs, standard-library HTTP/TLS/context handling, and a testable reconciliation core. It does not add a generic enterprise repository framework, interfaces for every struct, or a custom task scheduler. The consumer-owned service interface describes only the operations the service actually calls. [G2]

A configured backend's concurrency contract is now documented; that comment is not proof that every adapter satisfies it. `context.WithoutCancel` in process startup intentionally retains values while allowing readiness and RPC draining before stopping background work. Its child cancel function is explicitly owned. That choice still requires end-to-end shutdown verification. Request handlers continue to use their own cancellation/deadline paths. [G3–G4]

## Imported execution claims (not verified here)

The installed compiler is **Go 1.23.2**. The full module declares **Go 1.25.0 minimum**; existing CI/container configuration selects **1.26.8**. Those are different scopes. Passing dependency-free tests under 1.23.2 does not certify the target compiler, module graph, or production runtime.

| Check | Actual result |
|---|---|
| Original core race suite | 76 passing test/subtest events; 13 top-level test events; six tested packages. |
| Revised core race suite | 107 passing test/subtest events; 21 top-level events including two fuzz-seed parents; six tested packages. `cmd/certgen` also compiles and has no tests. |
| Core `go vet` | Passed for the same dependency-free package list. |
| `FuzzParseKey` | Passed, 133,450 executions in a short configured 3-second fuzz run. |
| `FuzzDecodeSetRequest` | Passed, 48,222 executions in a short configured 3-second fuzz run. |
| Formatting/shell syntax | Passed `scripts/check-format.sh` and `bash -n scripts/*.sh`. |
| Documentation-presence scan | Zero remaining findings in the scanner's scope. |
| Selected before/after regressions | Original fails unknown-code/privacy/cancellation regressions; revised core passes. |
| Full module build attempt | Blocked before compilation: installed compiler is below the module's required version. |
| Bootstrap prerequisite check | Correctly rejects absent `go.sum` and generated protobuf bindings. |

Counts include parent/subtest/fuzz-seed events; they are not a claim of 107 independent unit-test functions. Active fuzz runs were not run with the race flag. The fuzz seeds also ran as part of the normal race-enabled core suite. Passing race/fuzz tests covers observed executions, not every possible interleaving/input. [G6–G7]

The raw records and scanner source are under `docs/validation/review-2026-09-20/`. See [VALIDATION.md](VALIDATION.md) for commands and evidence mapping.

## Historical release recommendations

**Build and supply chain.** Generate the protobuf files with the real compiler/plugins; resolve and review `go.mod`/`go.sum`; run the complete build, tests and vet under the intended supported compiler. `go.sum` records module checksums; it is not a standalone lockfile or proof of a clean supply chain. The revised gates check consistency, not bit-for-bit reproducibility across arbitrary toolchains. Pin/review protoc and container base-image digests and CI action commit SHAs before a release. [G5]

**Security scan.** `govulncheck` is configured but was not executed. CI now selects the verified available tool version `v1.8.0`; that is a reproducible selection, not a claim that it will remain the latest. Scan both source/build configuration and release binaries when possible, and separately scan container OS packages. A clean Go vulnerability scan is not a general security certification. [G8]

**External adapters.** Run all Kubernetes fake-client and gRPC tests, including the added unknown-error regression. Validate generated field names, API versions, controller callbacks and all process startup/early-return paths. Full-module type checking is a blocking gate, not optional polish.

**Runtime/concurrency.** Exercise leadership loss, startup cancellation, certificate expiration/rotation, connection draining and goroutine termination. Kubernetes leader-election leases alone are not an application-level fencing design. Live reads across multiple resources are not one atomic snapshot. Existing HPA checks leave a race if ownership changes after the check.

**Evidence freshness and scale.** Informer initial synchronization must not be presented as a guarantee of current watch freshness. The rate-limited work queue is not capacity-bounded; deployment listing is not yet paginated. Add limits, explicit freshness/coverage metadata and load testing before multi-tenant exposure. A Kubernetes connectivity check is not a complete application-health assessment.

**Authorization.** The reference authenticates an allowed client identity; it is not a complete per-user, per-tenant, per-tool authorization system. A future approval gate in an MCP wrapper is insufficient while a privileged caller can bypass it through raw `SetReplicas`. The final mutation endpoint must enforce the approved-action contract. The development certificate generator is not a Teleport or SPIFFE deployment.

**Deployment.** Run Helm lint/rendering, container builds, KIND integration for levels 1–5 and both real-Service rolling-upgrade tests. Do not use employer production systems to complete these lab gates. No zero-downtime or production-readiness assertion is supported by this review.

## Current verification commands

The review narrative above is historical. Use the current
[development setup](docs/DEVELOPMENT.md) and [integration report](docs/INTEGRATION-VALIDATION.md)
to reproduce the supported checks from the repository root:

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/dev.sh" quality vuln docker-test
bash "$repo_root/scripts/dev.sh" integration-all
for level in 3 4 5; do
  bash "$repo_root/scripts/dev.sh" upgrade-test "LEVEL=$level" || exit 1
done
```

The current Makefile has no `fuzz-core` target. Generated bindings and `go.sum`
are already supplied; run `prepare` only after deliberately changing schemas,
generators or dependencies. The imported fuzz counts above are not current-tree
execution evidence. `quality` covers source/build checks; cluster tests are separate.

## Official references

Sources reviewed September 20, 2026. Guidance is attributed separately from this review's observations and recommendations.

- [G1: Effective Go](https://go.dev/doc/effective_go)
- [G2: Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [G3: Go Doc Comments](https://go.dev/doc/comment)
- [G4: context package](https://pkg.go.dev/context)
- [G5: Managing dependencies](https://go.dev/doc/modules/managing-dependencies) and [go.mod reference](https://go.dev/doc/modules/gomod-ref)
- [G6: Data Race Detector](https://go.dev/doc/articles/race_detector)
- [G7: Go fuzzing tutorial](https://go.dev/doc/tutorial/fuzz)
- [G8: Go Vulnerability Management](https://go.dev/doc/security/vuln/) and [govulncheck v1.8.0](https://pkg.go.dev/golang.org/x/vuln@v1.8.0/cmd/govulncheck)
- [G9: gRPC status package](https://pkg.go.dev/google.golang.org/grpc/status)
- [G10: encoding/json compatibility/security notes](https://pkg.go.dev/encoding/json)
- [G11: Go downloads/toolchain releases](https://go.dev/dl/)
- [T1: Teleport SRE challenge](https://github.com/gravitational/careers/blob/main/challenges/sre/challenge.md)
