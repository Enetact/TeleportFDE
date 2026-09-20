# Development setup and dependency map

This is a development/reference project. Its setup targets a local Kubernetes
lab and GitHub-hosted CI. `toolchain.env` contains the reviewed tool versions,
Linux amd64/arm64 download checksums, and multi-platform image digests.

## Windows 11 with existing Ubuntu WSL

Open PowerShell in this repository:

```powershell
$repoRoot = (Get-Location).Path
& (Join-Path $repoRoot 'scripts/setup-local.ps1')
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments prepare
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments format
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments quality,vuln
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments pull-images,docker-test,docker-build
```

The scripts derive paths from their own directory; the repository can move.
`-Distribution` selects another installed Ubuntu WSL distribution. Setup installs
Linux tools into the WSL user's data directory; it does not add a native Windows
`go.exe` to PATH. All commands above execute in that same Linux environment.

Setup installs the Ubuntu build tools (including GCC for `go test -race`), CA
certificates, curl, Git, jq, unzip and Python 3. Go, KIND, kubectl, Helm and protoc
archives are checked against committed SHA-256 values before installation.
The Go vulnerability scanner and workflow checker are installed at pinned module
versions using Go's module checksum verification. No shell profile is rewritten.

An existing Docker engine is reused. If absent on Ubuntu, setup adds Docker's
official apt repository and installs Engine, CLI, containerd, Buildx and Compose.
It does not uninstall conflicting packages, change Docker group membership,
replace an existing daemon configuration, or reset containers/volumes. Docker
access must already be available to the selected WSL user for build/cluster work.

## Linux and macOS

On Ubuntu Linux:

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/setup-dev.sh"
source "$repo_root/scripts/dev-env.sh"
make prepare
make format quality vuln
make pull-images docker-test docker-build
```

The automated installer supports Ubuntu on amd64 and arm64. On macOS, install the
versions from `toolchain.env` using the official vendor packages and a working
Docker engine; install Xcode command-line tools for race tests, GNU make, Bash,
Python 3, curl and jq. Use `make` from the repository root with those tools on PATH.
The macOS setup path has not been exercised by this change.

## Packages and build requirements

| Component | Selection | Required by |
|---|---|---|
| Go toolchain | 1.27.1 | Local/CI builds and the builder image |
| Minimum Go language/toolchain | 1.26.0 | Kubernetes v0.37.0 module requirements |
| gRPC Go | v1.83.2 (security backport) | Server, CLI and rollout probe |
| protobuf Go | v1.36.12 | Generated message runtime and protoc-gen-go |
| Kubernetes api/apimachinery/client-go | v0.37.0, aligned | Backend, informers, CRD client and election |
| protoc | 36.2 | Explicit binding regeneration |
| protoc-gen-go-grpc | v1.6.2 | gRPC service binding generation |
| KIND / kubectl | v0.33.0 / v1.37.0 | Local cluster and integration commands |
| Helm | v4.3.0 | Lint/render/package/deploy; uses Helm 4 rollback/wait flags |
| actionlint / govulncheck | v1.7.12 / v1.8.0 | Workflow syntax and Go vulnerability checks |
| Bash, GNU make, GCC, curl, jq, unzip, Git, Python 3, CA roots | Ubuntu packages | Bootstrap, race tests and automation |
| Docker Engine and Buildx | Existing verified installation, or official Ubuntu packages | Image builds, KIND nodes and probes |

`go.mod` selects the module graph and `go.sum` authenticates downloaded module
contents. Generated files under `gen/replicas/v1/` are included in source control.
`make prepare` is the explicit update/bootstrap operation. `make quality` does
not silently tidy dependencies or regenerate the working tree. It compares freshly
generated bindings in a temporary directory with the supplied files.

The gRPC selection intentionally uses the patched stable v1.83.2 backport.
On 2026-09-20, the Go vulnerability database still flags v1.84.0 for
[GO-2026-6443](https://pkg.go.dev/vuln/GO-2026-6443). The
[upstream security advisory](https://github.com/grpc/grpc-go/security/advisories/GHSA-2v4p-qf9q-27wj)
confirms v1.83.2 is patched. This avoids requiring the development pseudo-version
shown by the scanner. A future update must pass `make vuln`; do not restore
v1.84.0 solely because its version number is higher.

`make format` and `make format-check` both use `scripts/check-format.sh`.
It enumerates every tracked and nonignored untracked `.go` file, including `gen/`,
and handles spaces in filenames. Installed tools and scratch work stay ignored.
`go.mod` formatting is handled by `go mod tidy`, not by gofmt.

`make toolchain-check` validates cross-file package/generator/image alignment.
`make outdated` reports newer modules; it does not upgrade them automatically.
Review transitive updates in context rather than forcing every indirect module to
its newest release independently of the upstream dependency graph.

## Image requirements

| Image | Purpose and contents | Build/pull path |
|---|---|---|
| Pinned `golang:1.27.1-bookworm` | Compiler, C toolchain for race tests, CA roots; resolves Go modules from go.mod/go.sum | `make pull-images`; Dockerfile source/test/build stages |
| `replica-control:test` | Source plus dependencies; executes full race tests and vet | `make docker-test` |
| `replica-control:dev` | Three static Linux binaries and CA roots; non-root scratch runtime | `make docker-build` |
| Pinned `kindest/node:v1.37.0` | Kubernetes node/control-plane components | `make pull-images`; `make cluster` |
| Pinned `registry.k8s.io/pause:3.10.2` | Disposable integration-test Deployment | `make pull-images`; integration script |

The scratch runtime needs no Go installation, libc, shell, protoc or package
manager: binaries are built with CGO disabled. Development/test stages supply
the compilers and module downloads. TLS leaf identities are generated locally
and mounted as a Kubernetes Secret, never baked into an image. The application
ServiceAccount token is projected by Kubernetes. The probe uses the same image
with `/probe` as its command.

BuildKit supplies target OS/architecture; the compiler runs on the builder's
architecture. The source supports amd64 and arm64, but executed platform evidence
is listed separately in DEPENDENCY-VALIDATION.md. Docker does not need host Go,
but the checked-out source must contain its generated bindings and checksum file.

## Local cluster checks

```bash
source scripts/dev-env.sh
make integration-all
make upgrade-test LEVEL=3
make upgrade-test LEVEL=4
make upgrade-test LEVEL=5
```

These commands create/update only the named local KIND lab and its test resources.
The release stays running. Select `CLUSTER`, `NAMESPACE` and `RELEASE` overrides
before generating certificates. `make clean-cluster` deletes the explicitly named
lab cluster; it is not part of ordinary setup. The existing fixed-duration rollout
probe is sampled evidence and still has the coverage limitation described in A2
of the historical audit.

## Git workflow and GitHub Actions

Use `feature/*` branches for changes, PRs into `develop`, `release/*` for release
preparation, and `hotfix/*` for focused corrections. Merge release/hotfix results
back into the appropriate long-lived branches. This is a supported convention;
setup does not create branches or change repository protection settings.

CI runs on the configured Gitflow branches, PRs and `v*` tags. It installs the
same pinned tools, checks all Go formatting, verifies generated bindings, runs
race tests/vet/build, renders all five chart profiles, checks workflows, scans Go
vulnerabilities, and tests/builds the Docker image. Pull requests and pushes
automatically run the KIND matrix for levels 1–5 after quality passes; rollout
checks include 3–5. Matching branch and version-tag pushes are included. A push
to an open PR can run both matrices. For a manual workflow run, select the branch
and enable `cluster_tests` to run the same integration jobs.

Version tags package a Helm chart and local image archive as short-lived workflow
artifacts. Nothing is automatically deployed or pushed to a container registry.
Actions use reviewed full commit pins and read-only repository permissions.
Dependabot checks Go modules, actions and Docker references weekly after this
configuration is committed. Toolchain/image changes must also update the shared
pins/checksums and pass `make toolchain-check`. Remote runs and branch protections
are not verified merely by validating these workflow files locally.
