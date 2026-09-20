# Reading and explaining the code

This guide is a study path, not an interview script. Run the code, change it, and
explain the invariants in your own words before relying on it in an assessment.

## 1. Start with the contract, not Kubernetes

Read `internal/model/model.go` and `internal/service/service.go` first.
`Backend` is comparable to a small C# interface behind a service layer. The service
validates namespace, Deployment name, presence of replicas, bounds, and optional
version. HTTP and gRPC call the same behavior instead of duplicating business rules.

`*int32` represents presence. Nil means the caller omitted a value; a pointer to
zero means a deliberate scale-to-zero. This is why treating a missing integer's
zero value as a real request would be dangerous. The protobuf contract also uses
`optional int32` rather than an implicitly defaulted scalar.

Exercise: add a table-driven test for a 64-character namespace. Confirm the
request fails before your fake backend sees a call.

## 2. Trace one HTTP request

Read `internal/httpapi/http.go`. A GET path supplies a `Target`; the shared service
validates it; the backend returns a value object. A PUT requires JSON, rejects
unknown fields/trailing objects, limits body size, then delegates to the same core.
Responses translate semantic errors into stable HTTP status codes.

There is no HTTP framework dependency. `http.NewServeMux` handles method-aware
routes. `context.WithTimeout` carries a request budget into client-go, roughly the
role a CancellationToken/deadline plays across C# async calls.

Exercise: run the tests for missing replicas, explicit zero, conflict, and HPA
precondition failure. Explain why those are four different situations.

## 3. Understand direct Kubernetes scaling

Read `internal/kube/backend.go`, especially Set. The adapter retrieves the live
Deployment and current scale object, checks UID/resourceVersion, changes only the
replica field on a copy, and writes the scale subresource. A stale caller version
returns a conflict rather than silently winning. Unconditional conflicts may be
retried against a fresh version.

Do not mutate an object obtained from an informer lister. Those objects belong to
a shared cache and are read by other goroutines. Value conversion or DeepCopy is
not optional housekeeping; it is a concurrency boundary.

Exercise: explain the difference between a Deployment name and UID, then simulate
a delete/recreate with the same name. Why should an old intent not automatically
scale the new workload?

## 4. Add the informer mental model

At level 4, API reads come from a local watch-backed cache, not a new GET each time.
Client-go's informer handles list/watch maintenance. The API gets a fast local
read but accepts eventual consistency. A successful write may not be reflected
in the very next cache read.

Read the fake-client test that clears recorded actions, performs repeated reads,
and asserts no new Kubernetes GET/list actions occurred. That tests behavior,
not just the existence of a map called a cache. Watch updates are tested separately.
Those Kubernetes-dependent tests still need to be executed on a prepared machine.

Exercise: why does `HasSynced` establish initial readiness but not guarantee that
a watch is currently receiving every update? What additional freshness policy
would a stricter production service require?

## 5. Distinguish mTLS authentication from authorization

Read `internal/security/tls.go` and its executed handshake tests. A client first
needs a trusted, time-valid client certificate. It then needs the exact authorized
URI SAN. A certificate from the correct CA with the wrong URI is still rejected.

The authorization policy is intentionally coarse: one operator identity can scale
eligible targets across the cluster. It is not per-user, per-namespace RBAC. Do not
claim otherwise because mutual TLS is present.

Exercise: explain the wrong-URI, wrong-CA, wrong-server-name, expired-certificate,
and wrong-EKU tests. Which side should reject each handshake?

## 6. Trace a level-5 change end to end

Read `.proto`, the gRPC adapter, `internal/kube/intents.go`, then the pure
reconciliation engine. The flow is:

```text
SetReplicas → validate → check live target → persist ReplicaIntent
                                                ↓ watch event
                                         rate-limited work queue
                                                ↓ elected leader
                                        read intent and target
                                                ↓ check versions / UID / HPA
                                       update Deployment /scale
                                                ↓ watch events
                                        record observed status
```

The client receives acceptance after persistence, before eventual Pod readiness.
The controller is the agent that continuously reconciles external drift, not a
single function call masquerading as reconciliation.

`observedGeneration` says which intent generation was processed. `phase=Ready`
also requires matching desired/ready replicas and an observed Deployment generation.
A newer intent can invalidate an old status update; the version check prevents an
old pass from incorrectly reporting the new intent as complete.

Exercise: walk through an old reconciliation pass racing a new SetReplicas call.
There is no cross-object transaction. Explain where a stale value can briefly be
applied, why the next pass converges, and which claim of strong consistency would
be incorrect.

## 7. Explain the operational packaging

Read the Helm Deployment and `cmd/server/main.go`. Multiple Pods independently
serve APIs. A Lease chooses the controller worker leader, not the only Pod allowed
to answer requests. Readiness gates traffic; liveness should not turn a Kubernetes
outage into a restart storm. Graceful shutdown first withdraws readiness, then
drains bounded work.

Read `cmd/probe/main.go` and the integration script. The upgrade test uses Service
DNS from a Job rather than kubectl port-forward. This matters because port-forward
can fail when its selected Pod disappears even if the Service is healthy.

Exercise: explain why `maxUnavailable: 0` alone is not proof of zero downtime.
Then run the test and inspect the actual request/failure counts and longest call.

## 8. Be explicit about what remains unproven

This archive has passing core/race/TLS tests, not a passing live Kubernetes report.
Complete bootstrap, dependency review, full tests, chart checks, integration, and
rollout measurements before claiming it works end to end. A working demo still
is not a production security assessment.
