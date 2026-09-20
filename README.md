# Replica Control — Teleport SRE Levels 1–5 Reference

Independent, AI-generated educational implementation prepared for Jamie. This is
not an official Teleport solution, an approved design, or a claim of production
readiness. It is a reference to study, run, critique, and adapt.

**Validation boundary:** the dependency-free Go packages were actually tested
with `-race` in the authoring environment. The Kubernetes and gRPC adapters, full
binary build, Helm rendering, Docker image, and live-cluster workflows were not
executed there: the environment had Go 1.23.2, no dependency downloads, and no
Docker/Kubernetes/Helm/protoc. See [VALIDATION.md](docs/VALIDATION.md).

**One bootstrap step is required:** run `make prepare` on a networked development
machine. It generates the protobuf Go bindings and the real `go.sum`. Neither is
fabricated in this archive. Commit those outputs before using the CI workflow.
Do not treat the archive as a reproducibly locked release until that step and the
full verification commands have passed.

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
| Go | Minimum 1.25 for full application; builder and CI select 1.26.8 |
| Go for `test-core` only | 1.23 or newer, with a working C compiler for `-race` |
| make | GNU make; shell recipes use bash |
| Docker | BuildKit-capable Docker with a running daemon |
| kubectl | 1.35.0 baseline for the bundled local Kubernetes target |
| KIND | 0.31.0 baseline; node 1.35.0 pinned by digest in Makefile |
| Helm | Helm 3; CI uses 3.19.0 (Helm 4 CLI compatibility is not assumed) |
| protoc | Protocol Buffers compiler supporting proto3 optional fields; 3.21+ |
| curl, jq | HTTP and JSON integration assertions |

These are reference pins, not claims that each dependency is the latest security
patch. Review dependency/image advisories before deployment outside a lab. Tool
installation references are in [SOURCES.md](docs/SOURCES.md).

On Ubuntu/Debian, the supporting OS tools can be installed with:

```bash
sudo apt-get update
sudo apt-get install -y build-essential make protobuf-compiler curl jq
```

Install Go, Docker, kubectl, KIND, and Helm 3 using their official instructions.
On macOS, ensure your command-line compiler tools are available for race tests.
`make doctor` identifies missing tools and checks the Docker daemon.

## First build and tests

```bash
# From the extracted repository directory:
make test-core

# Requires Go 1.25+, protoc, and access to Go module downloads:
make prepare
make test
make vet
make build
make helm-check

# After reviewing the resolved dependency versions:
git add go.mod go.sum gen/replicas/v1/*.go
```

`make prepare` pins the Go code generators to the versions in Makefile. It runs
`protoc`, `go mod tidy`, and `go mod verify`. The CI check intentionally fails when
the generated source/real lockfile have not been committed, or regeneration changes
those committed files. Pin the protoc version itself and CI action commit SHAs
for stronger release reproducibility; the supplied workflow does not yet do so.

The Dockerfile expects a prepared source tree, including `go.sum` and `gen/`.
Use `make docker-build`, which runs preparation, rather than a bare first-run
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
and API compatibility still matter. `make upgrade-test` measures the real Service
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
