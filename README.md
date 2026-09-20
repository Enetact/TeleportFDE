# Replica Control — Teleport SRE Levels 1–5 Reference

Independent, AI-generated educational implementation prepared for Jamie. This is
not an official Teleport solution, an approved design, or a claim of production
readiness. It is a reference to study, run, critique, and adapt.

**Current development setup:** see [DEVELOPMENT.md](docs/DEVELOPMENT.md) for
Windows/WSL installation, the package/image map and Git workflow. Go/Kubernetes
tool versions and image digests are centralized in `toolchain.env`. Go modules
are selected in `go.mod`; optional visual-build packages are pinned separately
in `docs/visuals/tools/package.json` and its lockfile. Generated protobuf
bindings and `go.sum` are now included. `make prepare` intentionally refreshes
them; ordinary quality/build targets check the supplied files without regenerating.

Fresh validation results are recorded in [DEPENDENCY-VALIDATION.md](docs/DEPENDENCY-VALIDATION.md).
The [integration validation report](docs/INTEGRATION-VALIDATION.md) records passing
local KIND checks for levels 1–5 and rollout probes for levels 3–5 after the tunnel fixes.
Use the root [validation index](VALIDATION.md) to distinguish current results
from imported review notes and historical audit records.
The original [VALIDATION.md](docs/VALIDATION.md) describes the earlier authoring
environment and remains historical evidence.

**Visual guides:** open [the local visual field guide](docs/visuals/index.html)
in a browser for five architecture walkthroughs with animated SVGs, GIFs,
Mermaid sources and downloadable diagrams. Viewing works offline; see the
[visual guide instructions](docs/visuals/README.md) to rebuild the assets.

Teleport's public challenge recommends that candidates write their own design
and code, obtain design approval, and use reviewable pull requests. This reference
is not a substitute for that process. Do not represent it as independently
written or as reviewer-approved.

## Coverage by level

| Level | Behavior selected by `--level` / `make LEVEL=` | Main files |
|---|---|---|
| [1](levels/level-1/README.md) | HTTP GET replica count, tests, Docker, documented Kubernetes deployment | `internal/httpapi`, `Dockerfile`, Helm chart |
| [2](levels/level-2/README.md) | Adds HTTP PUT replicas, validation, concurrency preconditions, integration script | `internal/service`, `internal/kube/backend.go` |
| [3](levels/level-3/README.md) | Adds Deployment listing; uses shared health and rolling deployment support | `internal/health`, `charts/replica-control` |
| [4](levels/level-4/README.md) | Moves HTTP reads to a Deployment informer cache; retains shared mTLS | `internal/kube/backend.go`, `internal/security` |
| [5](levels/level-5/README.md) | gRPC; cached reads; per-Deployment ReplicaIntent CRD; drift reconciliation; leader election | `.proto`, `internal/grpcapi`, `internal/controller`, `internal/reconcile`, `internal/kube/intents.go` |

All levels use mTLS in this reference; there is deliberately no insecure API
mode. Levels 1–3 do not use the informer as their read path. Level 5 exposes gRPC
instead of also exposing the level 1–4 REST endpoints. Health probes remain HTTP
on a separate Pod-only port, not on the Service.

## Start here

Read [the design](docs/DESIGN.md), then follow the level sequence in
[WALKTHROUGH.md](docs/WALKTHROUGH.md). The source is intentionally layered:

```text
cmd/server/          Process lifecycle, TLS, HTTP/gRPC selection, leader election
cmd/replicactl/       Authenticated gRPC client
cmd/certgen/          Dependency-free local-development certificate generator
cmd/probe/           Real-Service rollout availability probe
cmd/tlscheck/        Host-side certificate rejection verifier
api/replicas/v1/      Complete Protocol Buffers API contract
internal/model/      Shared types, errors, limits, validation
internal/service/    Transport-independent API behavior
internal/httpapi/    HTTP routes, strict input decoding, status mapping
internal/grpcapi/    gRPC handlers, deadlines, error mapping
internal/kube/       client-go adapters, informer reads, scales, CRD persistence
internal/reconcile/  Testable controller decisions, independent of Kubernetes SDK
internal/controller/ Informer events, workers, retry/backoff queue
internal/health/     Separate liveness, readiness, and live dependency checks
internal/security/   TLS 1.3, certificate verification, URI SAN authorization
charts/replica-control/  Helm resources and structural CRD schema
scripts/             Integration and formatting checks
.github/workflows/   Quality checks and automatic PR/push cluster-test matrix
levels/level-1..5/   Guides, shared file maps, command wrappers and Helm overlays
gen/replicas/v1/     Committed generated protobuf and gRPC bindings
docs/visuals/        Offline HTML guides, Mermaid sources, SVGs and animated GIFs
```

## Windows quick start

Open PowerShell in the repository root (the directory containing this README).
Use the existing Ubuntu WSL distribution; the wrappers default to `Ubuntu-24.04`
and accept `-Distribution` for another installed Ubuntu distribution.

```powershell
$repoRoot = (Get-Location).Path
& (Join-Path $repoRoot 'scripts/setup-local.ps1')
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments quality,vuln
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments pull-images,docker-test,docker-build
Start-Process (Join-Path $repoRoot 'docs/visuals/index.html')
```

Setup installs the Linux Go toolchain and requirements inside WSL, not a native
Windows `go.exe`. Existing Docker is reused. Opening the visual guides needs only
a browser; Node.js is required only to rebuild their graphics. Paths are derived
from the selected repository directory and continue to work after relocation.

## Prerequisites

The exercised application workflow is Ubuntu 24.04 under WSL on arm64. Linux and
manually configured macOS are also supported setup paths; macOS was not exercised.
Docker must be running and accessible for image builds and the local KIND lab.

| Tool | Reference baseline / requirement |
|---|---|
| Go | Minimum 1.26 for Kubernetes clients; local tools, builder and CI select 1.27.1 |
| Go for `test-core` only | 1.23 or newer, with a working C compiler for `-race` |
| make | GNU make; shell recipes use bash |
| Docker | BuildKit-capable Docker with a running daemon |
| kubectl | 1.37.0 baseline for the bundled local Kubernetes target |
| KIND | 0.33.0 baseline; node 1.37.0 pinned by digest in toolchain.env |
| Helm | Helm 4.3.0; commands use rollback-on-failure and watcher waiting |
| protoc | Protocol Buffers compiler 36.2; Go plugins are pinned separately |
| curl, jq | HTTP and JSON integration assertions |
| Git, Python 3, CA roots, unzip, C compiler | Setup, mapping checks, downloads and race tests |
| actionlint / govulncheck | Installed versions from toolchain.env; required by quality / vuln |

gRPC is deliberately pinned to the patched stable v1.83.2 backport; the higher
v1.84.0 release was flagged by the vulnerability scan. The selection and dated
results are recorded in [dependency validation](docs/DEPENDENCY-VALIDATION.md).
Run the scan again when dependencies change.

For Ubuntu/Linux or a WSL terminal, start in the repository root:

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/setup-dev.sh"
source "$repo_root/scripts/dev-env.sh"
```

The installer supplies the OS tools and pinned toolchain. For manual macOS setup,
follow [DEVELOPMENT.md](docs/DEVELOPMENT.md), including compiler tools for race tests.
`make doctor` identifies missing tools and checks the Docker daemon.

## First build and tests

```bash
# From the repository root after setup:
repo_root="$(pwd)"
bash "$repo_root/scripts/dev.sh" quality vuln
bash "$repo_root/scripts/dev.sh" pull-images docker-test docker-build

# Repair Go formatting when needed, then check it:
bash "$repo_root/scripts/dev.sh" format format-check

# Optional smaller dependency-free test suite:
bash "$repo_root/scripts/dev.sh" test-core
```

The format inventory covers tracked and nonignored untracked Go files, including
the generated bindings. `quality` also verifies generation, runs race tests/vet,
builds all four host binaries, checks harness lifecycle, renders all five chart profiles, checks workflow syntax
and verifies module checksums. `vuln` remains a separate required security gate.

`make prepare` pins the Go code generators to the versions in toolchain.env. It runs
`protoc`, `go mod tidy`, and `go mod verify`. The quality check fails when
the generated source/checksum file is missing, or regeneration differs from
those supplied files. This revision pins protoc and CI action commit SHAs
alongside download checksums and builder/node/test image digests.

The Dockerfile expects a prepared source tree, including `go.sum` and `gen/`.
Run `make prepare` after intentionally changing dependencies or schemas; both
`make docker-build` and a direct build consume the prepared source tree:
`docker build .`.

After deliberately changing the schema, generator or dependency selection:

```bash
bash "$repo_root/scripts/dev.sh" prepare
bash "$repo_root/scripts/dev.sh" quality vuln
```

Review and commit the resulting `go.mod`, `go.sum` and generated-source changes
together. A normal first build uses the supplied files and does not need `prepare`.

## Deploy and exercise each level

```bash
make deploy LEVEL=1
make integration LEVEL=1

make integration LEVEL=2
make integration LEVEL=3
make integration LEVEL=4
make integration LEVEL=5

# Or run the sequence:
make integration-all

# Test upgrades using the real in-cluster Service:
make upgrade-test LEVEL=3
make upgrade-test LEVEL=4
make upgrade-test LEVEL=5
```

Defaults: KIND cluster `sre-reference`, namespace `replica-system`, Helm release
`replica-control`, and image `replica-control:dev`. Kubernetes commands explicitly
select `kind-sre-reference`; Helm uses that same context. KIND cluster creation may
add/switch a kubeconfig context as part of its normal behavior, but the scripts do
not rely on your current context.

For a distinct local cluster/release:

```bash
make deploy LEVEL=5 CLUSTER=my-sre-lab NAMESPACE=sre-lab RELEASE=replica-lab
```

Choose these names before creating local certificates. Existing certificates are
reused; they will not automatically gain SANs when a release/namespace changes.
Certificate rejection checks each use an isolated temporary tunnel; only a remote
TLS certificate alert counts as rejection. Authenticated API checks follow on a
fresh tunnel. Per-stage diagnostics are saved under
`artifacts/integration/level-N/` and uploaded by CI without certificate keys.
See [integration validation](docs/INTEGRATION-VALIDATION.md) for scope and results.

The integration script creates a unique temporary namespace, checks behavior,
then removes that namespace on success or failure. The application release is
left running for inspection.

Local test identities live under `.local/pki`, which is ignored by Git. Only the
server leaf key/certificate and public CA certificate are uploaded to the server
Secret. The CA private key is never uploaded. Leaf identities last seven days;
CA identity lasts thirty days. Existing files are not overwritten silently.

## HTTP examples — levels 1–4

After `make deploy LEVEL=4`, create a disposable target:

```bash
source scripts/dev-env.sh
kubectl --context kind-sre-reference create namespace replica-demo
kubectl --context kind-sre-reference -n replica-demo create deployment demo \
  --image="$PAUSE_IMAGE"

# Leave this command running in a separate terminal:
kubectl --context kind-sre-reference -n replica-system \
  port-forward service/replica-control 8443:8443
```

Then:

```bash
curl --fail --cacert .local/pki/ca.crt \
  --cert .local/pki/client.crt --key .local/pki/client.key \
  https://localhost:8443/v1/namespaces/replica-demo/deployments/demo/replicas

curl --fail --cacert .local/pki/ca.crt \
  --cert .local/pki/client.crt --key .local/pki/client.key \
  -X PUT -H 'Content-Type: application/json' -d '{"replicas":3}' \
  https://localhost:8443/v1/namespaces/replica-demo/deployments/demo/replicas

curl --fail --cacert .local/pki/ca.crt \
  --cert .local/pki/client.crt --key .local/pki/client.key \
  'https://localhost:8443/v1/deployments?namespace=replica-demo'
```

For a conditional write, take the GET response's `resourceVersion` and send it
as `expectedVersion`. A mismatch returns HTTP 409. An omitted field is an
unconditional write. A missing `replicas` is an error; zero is a valid request.
Replica requests outside 0–1000 are rejected.

## gRPC examples — level 5

```bash
make deploy LEVEL=5
make build
# Create replica-demo/demo as above if it does not exist.
# Restart the port-forward if an earlier upgrade replaced its selected Pod.

.bin/replicactl get replica-demo demo
.bin/replicactl set replica-demo demo 3
.bin/replicactl list
.bin/replicactl list replica-demo
.bin/replicactl health

kubectl --context kind-sre-reference -n replica-demo get replicaintents
kubectl --context kind-sre-reference -n replica-demo get replicaintent demo -o yaml

# Introduce drift; the elected controller should restore the persisted target:
kubectl --context kind-sre-reference -n replica-demo scale deployment/demo --replicas=1
```

Global flags must precede the CLI command. A conditional gRPC write uses the
intent's `intentVersion`, **not** the Deployment's `resourceVersion`:

```bash
.bin/replicactl --expected-version '<intentVersion-from-get>' \
  set replica-demo demo 4
```

At level 5, Set returns `accepted: true` once the intent is persisted. That is not
a declaration that the Deployment or its Pods already match. GET returns the
cached Deployment count, desired intent, ready count, and reconciliation phase.
Poll until the appropriate state is observed. A status update can also advance
an intent's resourceVersion; handle a version conflict by reading it again.

The full protobuf contract is `api/replicas/v1/replicas.proto`; gRPC reflection is
not enabled. The bundled client means grpcurl is not an extra requirement.

## Health and safe upgrades

`/livez` means the process can respond. `/readyz` and `/healthz` additionally require
initial cache synchronization (where used) and a recent successful Kubernetes
probe. Connectivity checks run separately every three seconds, not on every GET.
A failed check removes readiness; it does not provoke a liveness restart loop.
These HTTP routes are on port 8081, omitted from the Service. Level 5 additionally
implements the standard authenticated gRPC health service.

The Helm Deployment defaults to two replicas, `maxUnavailable: 0`, `maxSurge: 1`,
readiness/startup probes, `minReadySeconds: 5`, and a PDB. A ready Pod may receive
Service traffic immediately; the five-second stability period controls when the
Deployment counts it available for rollout progress. API Pods do
not need controller leadership to serve requests. Shutdown marks readiness false,
waits for endpoint propagation, then drains HTTP/gRPC work with a deadline.

This is a design intended for safe rolling upgrades, **not an unconditional
zero-downtime guarantee**. Capacity, scheduling, TLS trust overlap, network behavior,
and API compatibility still matter. `make upgrade-test` samples the real Service
with fresh authenticated connections from a Job; any failed request fails the test.
The in-cluster probe starts with a successful authenticated request, observes the
whole Helm upgrade, and continues for ten seconds after explicit completion.
Its 300-second deadline fails closed if completion is missing or late. Availability
is sampled with a three-second request deadline, not a guarantee of zero latency.
See [integration validation](docs/INTEGRATION-VALIDATION.md) and the
[rollout visual guide](docs/visuals/rollout.html).

## Validation and workflow status

The [2026-09-20 local checks](docs/DEPENDENCY-VALIDATION.md) passed formatting for
24 Go files, source and compiled-server vulnerability scans, race tests, vet,
builds, generated-code verification, chart/workflow checks, Docker tests and local
packaging. This is arm64 WSL evidence; remote CI and live KIND acceptance are
separate checks. The [visual guide checks](docs/visuals/VALIDATION.md) cover offline
HTML loading, desktop/mobile layouts, playback, GIF motion and reduced motion.

GitHub Actions covers the configured Gitflow branches, PRs and version tags.
Version tags package the chart and image archive. Pull requests and pushes
automatically run the levels 1–5 cluster matrix after quality passes, with rollout
probes for levels 3–5. This includes matching branch pushes and version-tag pushes.
A push to an open PR can therefore produce both push and PR integration runs.
Manual runs can also enable `cluster_tests`. See
[DEVELOPMENT.md](docs/DEVELOPMENT.md) for the branch conventions and exact commands.

## Important boundaries

An authorized client has coarse cluster-wide scaling rights. The service denies
system namespaces, annotated protected Deployments, and HPA-managed Deployments.
These are application checks, not a substitute for Kubernetes authorization.
The chart's workload itself is marked protected.

Deploy only one replica-control release per cluster. Multiple independently
elected releases sharing the same CRD domain are not an isolation mechanism.
Do not run another HPA or replica-setting GitOps controller against a managed
Deployment. HPA detection is a useful guard but not an atomic Kubernetes ownership
lock. CRD intent and Deployment updates are also not a cross-resource transaction:
a stale value can be briefly applied during a concurrent change, followed by
another reconciliation pass.

TLS certificate/CA material is loaded on startup; live certificate reload and CRL/
OCSP revocation are not implemented. Rotate through a controlled rolling restart
with overlapping trust where required. Generating a new development CA is not a
zero-downtime production rotation procedure.

Reads are eventually consistent. Full-cluster listing is not API-paginated in
this reference, and gRPC responses are capped at 8 MiB. Scope-limited authorization,
large-cluster pagination, detailed metrics, and production PKI are further work.
See [SECURITY.md](docs/SECURITY.md) and the design's tradeoffs.

## Cleanup

```bash
# Remove only the disposable example target:
kubectl --context kind-sre-reference delete namespace replica-demo

# Destructive to the named local cluster; does not target your current context:
make clean-cluster CLUSTER=sre-reference

# Remove local build caches; development PKI is deliberately retained:
make clean
```

Helm does not automatically upgrade or remove CRDs placed in `crds/`. The supplied
local deployment target applies the current CRD explicitly before Helm. For a real
release, handle CRD schema migration and uninstall retention as separate, reviewed
operations. Do not remove the CRD casually: doing so removes its persisted intents.
