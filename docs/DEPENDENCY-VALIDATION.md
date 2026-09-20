# Dependency and development automation validation

Checked on 2026-09-20 in Ubuntu 24.04 under WSL, Linux arm64. This report
supplements the original source audit; it does not turn the historical audit
or imported review notes into evidence of live cluster behavior.

This records the earlier dependency-repair pass. The later
[integration report](INTEGRATION-VALIDATION.md) adds passing real KIND checks for
levels 1–5, rollout checks for 3–5, the fourth host binary (`tlscheck`), and a
29-file formatting inventory. Image IDs and the 24-file count below belong to
the earlier pass and are not identifiers/counts for the updated runtime.

## Installed and mapped requirements

The installed toolchain is Go 1.27.1, protoc 36.2, protoc-gen-go v1.36.12,
protoc-gen-go-grpc v1.6.2, KIND v0.33.0, kubectl v1.37.0, Helm v4.3.0,
actionlint v1.7.12 and govulncheck v1.8.0. Ubuntu supplies GCC/build-essential,
CA roots, curl, Git, jq, unzip and Python 3. The existing Docker 29.7.2 engine
and Buildx 0.36.1 are reused. Newer daemon packages were available; the installer
does not upgrade an existing daemon or interrupt other local workloads.

All tool and image pins are in `toolchain.env`; the dependency graph is recorded
in `go.mod` and `go.sum`. Source-controlled protobuf bindings are in
`gen/replicas/v1/`. Setup and execution resolve their own repository directory.
Windows uses the PowerShell wrappers and WSL; native Windows Go was not installed.

The gRPC runtime is intentionally pinned to **v1.83.2**, an upstream stable
security backport. The initially selected v1.84.0 failed the vulnerability gate
for [GO-2026-6443](https://pkg.go.dev/vuln/GO-2026-6443). The
[maintainer advisory](https://github.com/grpc/grpc-go/security/advisories/GHSA-2v4p-qf9q-27wj)
lists v1.83.2 as patched. Selecting that release also selected its required
`golang.org/x/net` v0.58.0 and `golang.org/x/text` v0.41.0. The vulnerability
gate remains mandatory; no advisory has been suppressed.

## Validation evidence

Local raw logs are retained under the ignored `artifacts/validation/` directory.

| Check | Result |
|---|---|
| Installed tool versions and archive SHA-256 checks | Passed |
| Module resolution and `go mod verify` | Passed |
| Unified Go formatting inventory | Passed for all 24 files, including generated code |
| Formatter discovery and portability | Passed: untracked file, filename with spaces, invocation outside the checkout, write mode and Windows wrapper |
| Patched gRPC source vulnerability scan | Passed: no vulnerabilities found |
| Shell syntax, PowerShell parsing, workflow syntax | Passed |
| Shared toolchain/image/generator mapping | Passed |
| Pinned Go builder, KIND node and pause image pulls | Passed on arm64 |
| Full patched source quality | Passed: generated-code comparison, race tests, vet, three builds, five chart renders, lint and module verification |
| Patched Docker test stage | Passed: full race tests and vet |
| Development runtime image | Built successfully; arm64; user 65532:65532; `--version` returned `dev` |
| Compiled server dependency and vulnerability scan | Embedded gRPC v1.83.2 confirmed; no vulnerabilities found |
| Local development packages | Helm chart and Docker image archive created successfully |

The validated runtime image ID is
`sha256:bce0981c45059c6bada2415f51073f7d7c10dca56976866b5e1d60076d00f337`.
The local test image ID is
`sha256:0b135db5ea0a542502b79be6274aace4d5df7c26e362615b80596977a7cd81ff`.
Packaging produced `artifacts/replica-control-0.1.0.tgz` and
`artifacts/replica-control-image.tar.gz`; these are ignored local build outputs.
The relevant receipts are `quality-patched.log`, `docker-test-patched.log`,
`docker-build.log`, `container-modules.txt`, `container-vuln.log` and `package.log`
under `artifacts/validation/`. The test stage in `docker-build.log` predates the
gRPC fix; `docker-test-patched.log` is the authoritative patched test-stage run.

The module update report reviewed 108 modules in the selected graph. The only
direct dependency with a numerically newer release was gRPC v1.84.0, intentionally
excluded for the advisory above. Updates are available for 59 indirect modules;
these were reviewed as dependency-graph information, not forced independently
above the versions selected by upstream packages. The clean vulnerability scan
applies to the selected graph, not every available module release.

`make quality` verifies formatting, generated bindings, race tests, vet, all
four host binaries, harness lifecycle regressions, Helm lint/render for levels 1–5, workflow syntax and module
checksums. `make vuln` scans the application dependency graph. `make docker-test`
runs tests/vet with the builder's requirements; `make docker-build` creates the
non-root image with static binaries and CA roots. The scratch runtime needs no
host Go installation or package manager.

## Remaining scope

GitHub-hosted execution, branch protections, native Windows/macOS builds,
amd64 execution remain separate checks. Local validation of YAML is not proof of
a remote CI run. Live KIND integration for levels 1–5 and rollout checks for 3–5
subsequently passed on Linux/ARM64. Historical findings A2 (probe coverage) and A4
(negative TLS-test evidence) are addressed by completion-controlled monitoring,
isolated tunnels, validated fixtures and explicit remote certificate alerts.
See the [integration receipt](validation/integration-summary.json). Invalid
rollout levels are rejected before deployment prerequisites run.

The original audit's missing-build-input finding is addressed by generating
and including the real checksum file and bindings. See
[DEVELOPMENT.md](DEVELOPMENT.md) for exact setup commands and the complete
package/image map.

## Official version sources

- [Go downloads](https://go.dev/dl/)
- [gRPC security backport release](https://github.com/grpc/grpc-go/releases/tag/v1.83.2)
- [Go protobuf releases](https://github.com/protocolbuffers/protobuf-go/releases)
- [Kubernetes client-go releases](https://github.com/kubernetes/client-go/releases)
- [Protocol Buffers compiler releases](https://github.com/protocolbuffers/protobuf/releases)
- [KIND releases](https://github.com/kubernetes-sigs/kind/releases)
- [Helm releases](https://github.com/helm/helm/releases)
- [Docker Ubuntu installation](https://docs.docker.com/engine/install/ubuntu/)
