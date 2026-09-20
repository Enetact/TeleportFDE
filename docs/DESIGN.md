# RFD: Replica Control reference architecture

**Status:** educational reference, not approved by a Teleport interview panel.
**Scope:** the five levels in the linked public challenge, implemented with one
shared core and a runtime level selector. For an actual assessment, choose only
the agreed level; do not submit the entire superset without discussing scope.

## Goal and invariants

Expose replica reads and controlled writes without hiding the distinction between
Kubernetes desired replicas (`spec.replicas`) and ready/available Pods. Higher
levels use local informer caches and, at level 5, persistent controller-owned
intent. A successful level-5 Set means durable intent, not completed scheduling.

Invariant: each managed Deployment instance has at most one same-name, same-namespace
ReplicaIntent, tied to its immutable UID. Never automatically transfer intent to a
same-name replacement. A malformed request cannot become an accidental scale-to-zero.
Only an authenticated and URI-authorized client may access the operational API.

## API structure

| Transport | Operation | Meaning |
|---|---|---|
| HTTPS | GET `/v1/namespaces/{namespace}/deployments/{name}/replicas` | Deployment spec/ready/available replica counts |
| HTTPS | PUT the same route | Set replicas directly at levels 2–4 |
| HTTPS | GET `/v1/deployments?namespace=...` | List; omitted namespace selects the whole cluster |
| gRPC | GetReplicas | Cached Deployment plus cached desired intent where present |
| gRPC | SetReplicas | Persist ReplicaIntent; return its version and acceptance |
| gRPC | ListDeployments | Cached listing, optionally scoped by namespace |

PUT accepts one JSON object: `replicas` is a required integer, including zero;
`expectedVersion` is optional. Counts are bounded to 0–1000, names validated, unknown
JSON fields rejected, bodies limited to 4096 bytes, and request work deadline-bound.

Both APIs return semantic errors: invalid argument, not found, forbidden,
failed precondition, conflict, unavailable, or internal. Backend details are not
returned verbatim. A conditional write is optimistic concurrency control; an
unconditional write is last-successful-write-wins. In level 5, version means intent
resourceVersion, including status-related updates. In lower levels, it means
Deployment resourceVersion, carried through the scale subresource.

## Implementation and caching

HTTP/gRPC adapters call a shared validation service. A client-go adapter implements
the backing operations. Levels 1–3 read live data. Levels 4–5 use shared Deployment
informers and listers; level 5 has a separate dynamic informer for ReplicaIntents.
No external cache or database is needed. List/watch reconnect and relist behavior
is delegated to client-go instead of a custom Kubernetes watch implementation.

Read-only API calls do not call the cluster at cached levels. Background watches,
controller operations, and explicit health probes do. GET/list see eventual state;
Deployment and intent caches are not one atomically consistent snapshot. A set
followed immediately by a read may still show the previous observed state.

The informer cache is per Pod and contains cluster-wide Deployments. This is a
small-cluster reference, not a large multi-tenant management plane. Full listing
is bounded by server response limits, not user-facing pagination.

## Level-5 controller and conflicts

Set first reads the live Deployment, rejects protected/HPA-managed targets, and
creates or updates its intent with optimistic resourceVersion checks. The CRD's
structural schema requires a matching resource name, immutable Deployment name and
UID, and bounded replicas. A Deployment owner reference supports garbage collection.

Informer events enqueue a `namespace/name` key. Two workers in the elected leader
reconcile idempotently. Each pass reads the current intent and target, checks UID,
deletion, protection and HPA state, then updates only the scale subresource. Intent
is reread before changing scale to reduce a concurrent-change window. Scale UID
and resourceVersion are rechecked; conflicts retry with rate-limited backoff.

There is no transaction spanning intent and Deployment. A concurrent new target
can race an old pass; later events converge to the newer intent. Status updates
check the original intent UID/generation before recording observedGeneration, and
skip identical updates to avoid event loops. Status distinguishes Ready,
Reconciling, and Blocked, with reasons such as HPAConflict or DeploymentUIDMismatch.
A periodic 30-second requeue catches missed events and revisits blocked intents.

Leadership uses a namespaced Lease. Every Pod maintains its own cache and serves
APIs; only the leader runs workers. Loss of leadership removes readiness and exits
the process. ReleaseOnCancel is false so the lease is not released before old work
stops; failover may wait for expiry. Leader election is not strict fencing, so
reconciliation must remain idempotent and version-aware even with rare overlap.

HPA checks do not establish exclusive ownership atomically. Running a competing
HPA/GitOps replica controller is explicitly unsupported. Remove that ownership
before creating an intent. A native policy object may also be removed to stop
this controller managing the target; removal leaves the last replica count intact.

## Security and TLS configuration

Use TLS 1.3 for all operational API levels. Require normal certificate chain,
validity, and client-auth EKU checks, then require an exact authorized client URI
SAN. The development identity is `spiffe://replica-control.local/client/operator`;
this is a URI identifier, not a full SPIFFE workload-identity deployment.

For TLS 1.3, Go selects its supported cipher suites rather than using
`Config.CipherSuites`: TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384, and
TLS_CHACHA20_POLY1305_SHA256. The implementation deliberately does not enable
legacy TLS 1.2. Clients verify server DNS/IP SANs; there is no InsecureSkipVerify.

A local generator issues P-256 certificates with purpose-specific EKUs and short
lifetimes. Secrets are generated outside Git, and the server never receives the
CA private key. Server key and CA changes require a rolling restart. Hot reload,
external CA integration, certificate revocation, fine-grained user authorization,
and CA-overlap rotation automation are not implemented.

ClusterRole grants only named API resources/verbs needed by this implementation;
a namespace Role limits Lease operations to the service's election namespace.
Scope is nevertheless broad: the single authorized operator can scale eligible
Deployments across namespaces. System namespaces and annotated protected targets
are denied by application logic; Kubernetes credentials must still be protected.

## Pod lifecycle and availability

Startup readiness waits for initial caches and a recent Kubernetes connectivity
check. Liveness does not depend on Kubernetes; a dependency outage should not
cause cascading restarts. The health listener is omitted from the Service and
only returns status, not application data. gRPC health is also available over mTLS.

The chart defaults to two API Pods, RollingUpdate maxUnavailable=0/maxSurge=1,
readiness/startup probes, minReadySeconds=5, a PDB, and preferred anti-affinity.
SIGTERM removes readiness, allows endpoint propagation, drains in-flight requests,
and enforces an upper shutdown deadline. Controller leadership is not a readiness
requirement, so both old and new API Pods can serve during controller handover.

Availability still assumes spare scheduling capacity, compatible API versions,
working networking, and overlapping TLS trust. A zero-error rollout claim requires
an actual measurement, not just a correct-looking Deployment manifest.

## Developer workflow, build and release

`make prepare` regenerates protobuf bindings and resolves/verifies module checksums.
The generated bindings and `go.sum` are already supplied; ordinary checks do not
require regeneration. `go.mod` selects dependencies and `go.sum` verifies downloads.
`make test` runs unit/fake-client tests; `make test-core` is the smaller offline
suite. `make deploy LEVEL=N` builds and loads a static, non-root image into a named
local KIND cluster and installs the chart. Docker produces one architecture per
invocation through BuildKit target variables, supporting amd64/arm64 workflows.

`make build` produces four host binaries; `tlscheck` is a development-only
certificate rejection verifier. The runtime image contains the server, gRPC CLI
and probe. Each deliberate TLS rejection gets a separate port-forward and must
return a remote certificate alert; broken tunnels and invalid fixtures fail.

`make integration-all` attempts all levels and fails if any level fails.
`make upgrade-test LEVEL=3` (also supported for 4 and 5) creates
an authenticated probe Job in the cluster that connects to the Service using fresh
connections while Helm rolls the application. A successful first request starts
monitoring; the harness acknowledges Helm completion through `/probe --finish`
inside the Pod, followed by ten seconds of additional sampling. The 300-second
probe deadline fails closed without that acknowledgment and observation period.
The Job allows 360 seconds and Helm allows 180 seconds. Any failed request fails the Job.
A kubectl port-forward is not used as the availability measurement because it
selects a specific Pod. Quality CI runs on pushes/PRs. Both push and pull-request
runs automatically test all five levels after quality passes. Manual runs can
also enable `cluster_tests`. A push to an open PR can trigger both matrices.
Selected per-level diagnostic logs are uploaded for seven days before cleanup;
private keys and Secret manifests are excluded from the artifact inputs.

Generated files and real go.sum must be reviewed and committed after bootstrap.
The protoc version, action commit SHAs and builder/node/test image digests are
pinned. `make vuln` is a separate required Go vulnerability gate. These checks
do not provide a signed-release supply chain. Current local integration and
rollout results are recorded in [INTEGRATION-VALIDATION.md](INTEGRATION-VALIDATION.md).

## Proposed review sequence

1. Agree a target level and submit the candidate's own design for review.
2. Review API/service behavior, errors, TLS, tests, and basic packaging.
3. For the higher agreed level, review caching, controller conflicts, operational
   automation, and actual integration/rollout results.

## Complete protobuf contract

```proto
syntax = "proto3";
package replicas.v1;
option go_package = "example.com/replica-control/gen/replicas/v1;replicasv1";

service ReplicaService {
  rpc GetReplicas(GetReplicasRequest) returns (Deployment);
  rpc SetReplicas(SetReplicasRequest) returns (SetReplicasResponse);
  rpc ListDeployments(ListDeploymentsRequest) returns (ListDeploymentsResponse);
}
message GetReplicasRequest { string namespace = 1; string name = 2; }
message SetReplicasRequest {
  string namespace = 1;
  string name = 2;
  optional int32 replicas = 3; // Presence distinguishes missing from an intentional zero.
  string expected_version = 4; // ReplicaIntent resourceVersion, not Deployment version.
}
message ListDeploymentsRequest { string namespace = 1; } // Empty means all namespaces.
message Deployment {
  string namespace = 1;
  string name = 2;
  string uid = 3;
  string resource_version = 4;
  int32 replicas = 5;
  int32 ready_replicas = 6;
  int32 available_replicas = 7;
  optional int32 desired_replicas = 8;
  string intent_version = 9;
  string reconcile_phase = 10;
  int64 generation = 11;
  int64 observed_generation = 12;
}
message SetReplicasResponse {
  string namespace = 1;
  string name = 2;
  int32 replicas = 3;
  string version = 4;
  bool accepted = 5; // Persisted intent, NOT a guarantee that Pods are ready.
}
message ListDeploymentsResponse { repeated Deployment deployments = 1; }

```
