# Replica Control — Teleport SRE Levels 1–5 Reference

Independent, AI-generated educational implementation prepared for Jamie. This is
not an official Teleport solution, an approved design, or a claim of production
readiness. It is a reference to study, run, critique, and adapt.

**Current development setup:** see [DEVELOPMENT.md](docs/DEVELOPMENT.md) for
Windows/WSL installation, the package/image map and Git workflow. All versioned
tools and image digests are centralized in `toolchain.env`. Generated protobuf
bindings and `go.sum` are now included. `make prepare` intentionally refreshes
them; ordinary quality/build targets check the supplied files without regenerating.

Fresh validation results are recorded in [DEPENDENCY-VALIDATION.md](docs/DEPENDENCY-VALIDATION.md).
The original [VALIDATION.md](docs/VALIDATION.md) describes the earlier authoring
environment and remains historical evidence.

Teleport's public challenge recommends that candidates write their own design
and code, obtain design approval, and use reviewable pull requests. This reference
is not a substitute for that process. Do not represent it as independently
written or as reviewer-approved.

## Coverage by level

| Level | Behavior selected by `--level` / `make LEVEL=` | Main files |
|---|---|---|
| 1 | HTTP GET replica count, tests, Docker, documented Kubernetes deployment | `internal/httpapi`, `Dockerfile`, Helm chart |
| 2 | Adds HTTP PUT replicas, validation, concurrency preconditions, integration script | `internal/service`, `internal/kube/backend.go` |
| 3 | Adds deployment listing across namespaces, live connectivity health, rolling Helm deployment | `internal/health`, `charts/replica-control` |
| 4 | Deployment informer cache, watch updates, mTLS, Make automation | `internal/kube/backend.go`, `internal/security` |
| 5 | gRPC; cached reads; per-Deployment ReplicaIntent CRD; drift reconciliation; leader election | `.proto`, `internal/grpcapi`, `internal/controller`, `internal/reconcile`, `internal/kube/intents.go` |

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
.github/workflows/   Unit checks and opt-in cluster-test matrix
```

## Prerequisites

Use macOS or Linux for the local-cluster workflow. A Linux development environment
is also the practical route on a Windows workstation. Docker must already be
running and able to create containers. Use an isolated development cluster, not
an employer's or production cluster.

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

These are reference pins, not claims that each dependency is the latest security
patch. Review dependency/image advisories before deployment outside a lab. Tool
installation references are in [SOURCES.md](docs/SOURCES.md).

On Ubuntu/Debian, the supporting OS tools can be installed with:

```bash
sudo apt-get update
sudo apt-get install -y build-essential make curl jq unzip python3
```

Run `bash scripts/setup-dev.sh` in Ubuntu/WSL, then `source scripts/dev-env.sh`.
On macOS, ensure your command-line compiler tools are available for race tests.
`make doctor` identifies missing tools and checks the Docker daemon.

## First build and tests

```bash
# From the repository directory after setup:
source scripts/dev-env.sh
make test-core

# Requires the pinned Go 1.27.1 toolchain, protoc, and access to Go module downloads:
make prepare
make test
make vet
make build
make helm-check

# After reviewing the resolved dependency versions:
git add go.mod go.sum gen/replicas/v1/*.go
```

`make prepare` pins the Go code generators to the versions in toolchain.env. It runs
`protoc`, `go mod tidy`, and `go mod verify`. The quality check fails when
the generated source/checksum file is missing, or regeneration differs from
those supplied files. This revision pins protoc and CI action commit SHAs
alongside download checksums and builder/node/test image digests.

The Dockerfile expects a prepared source tree, including `go.sum` and `gen/`.
Run `make prepare` after intentionally changing dependencies or schemas; both
`make docker-build` and a direct build consume the prepared source tree:
`docker build .`.

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
kubectl --context kind-sre-reference create namespace replica-demo
kubectl --context kind-sre-reference -n replica-demo create deployment demo \
  --image=registry.k8s.io/pause:3.10

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
readiness/startup probes, a five-second readiness window, and a PDB. API Pods do
not need controller leadership to serve requests. Shutdown marks readiness false,
waits for endpoint propagation, then drains HTTP/gRPC work with a deadline.

This is a design intended for safe rolling upgrades, **not an unconditional
zero-downtime guarantee**. Capacity, scheduling, TLS trust overlap, network behavior,
and API compatibility still matter. `make upgrade-test` samples the real Service
with fresh authenticated connections from a Job; any failed request fails the test.
Those rollout tests were supplied but not executed in the authoring environment.

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
