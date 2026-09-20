# Level 5 file map

All links below resolve to shared source. Files can serve multiple levels.

## Shared foundation

| Path | Responsibility |
|---|---|
| [cmd/server/main.go](../../cmd/server/main.go) | Process startup, level selection, Kubernetes clients and shutdown |
| [internal/model/model.go](../../internal/model/model.go) | Targets, replica limits, error contracts and backend interfaces |
| [internal/service/service.go](../../internal/service/service.go) | Transport-independent validation |
| [internal/kube/backend.go](../../internal/kube/backend.go) | Direct or cached Deployment reads and scale operations |
| [internal/security/tls.go](../../internal/security/tls.go) | TLS configuration used at every level in this reference |
| [internal/health/health.go](../../internal/health/health.go) | Background connectivity checks and probe endpoints |
| [cmd/certgen/main.go](../../cmd/certgen/main.go) | Local certificate generator |
| [Dockerfile](../../Dockerfile) | Shared image build; requires generated source and go.sum |
| [Makefile](../../Makefile) | Shared build, test, KIND and Helm commands |
| [go.mod](../../go.mod) | Full application dependency declarations |
| [go.offline.mod](../../go.offline.mod) | Dependency-free test subset only |
| [charts/replica-control](../../charts/replica-control) | Shared chart, values, workload, service, service account and RBAC |
| [.github/workflows/ci.yaml](../../.github/workflows/ci.yaml) | Quality gate, automatic PR/push cluster matrix and manual opt-in |
| [scripts/integration.sh](../../scripts/integration.sh) | Level-selected live checks |
| [scripts/check-format.sh](../../scripts/check-format.sh) | Go formatting check |
| [docs/DESIGN.md](../../docs/DESIGN.md) | Existing educational design and complete protobuf contract |
| [docs/VALIDATION.md](../../docs/VALIDATION.md) | Historical validation boundary |
| [docs/SECURITY.md](../../docs/SECURITY.md) | Security assumptions and limitations |

## Level implementation and tests

- [api/replicas/v1/replicas.proto](../../api/replicas/v1/replicas.proto)
- [gen/README.md](../../gen/README.md)
- [internal/grpcapi/grpc.go](../../internal/grpcapi/grpc.go)
- [internal/grpcapi/grpc_test.go](../../internal/grpcapi/grpc_test.go)
- [internal/kube/intents.go](../../internal/kube/intents.go)
- [internal/kube/backend_test.go](../../internal/kube/backend_test.go)
- [internal/controller/controller.go](../../internal/controller/controller.go)
- [internal/reconcile/reconcile.go](../../internal/reconcile/reconcile.go)
- [internal/reconcile/reconcile_test.go](../../internal/reconcile/reconcile_test.go)
- [internal/security/tls_test.go](../../internal/security/tls_test.go)
- [internal/health/health_test.go](../../internal/health/health_test.go)
- [internal/service/service_test.go](../../internal/service/service_test.go)
- [internal/model/model_test.go](../../internal/model/model_test.go)
- [cmd/replicactl/main.go](../../cmd/replicactl/main.go)
- [cmd/probe/main.go](../../cmd/probe/main.go)
- [charts/replica-control/crds/replicaintents.yaml](../../charts/replica-control/crds/replicaintents.yaml)

Generated build inputs are included at [replicas.pb.go](../../gen/replicas/v1/replicas.pb.go) and [replicas_grpc.pb.go](../../gen/replicas/v1/replicas_grpc.pb.go). All server levels require these because they share one binary. Use `make verify-generated` to check them against the pinned generators without rewriting the working tree.

Shared integration helpers: [tunnel lifecycle](../../scripts/port-forward.sh),
[TLS rejection verifier](../../cmd/tlscheck/main.go), and
[harness regression checks](../../scripts/test-harness.sh).

Additional shared verifier source and tests: [rejection.go](../../internal/security/rejection.go)
and [rejection_test.go](../../internal/security/rejection_test.go).
Current results: [integration report](../../docs/INTEGRATION-VALIDATION.md) and
[source-hash receipt](../../docs/validation/integration-summary.json).

Rollout lifecycle: [monitor.go](../../cmd/probe/monitor.go) and
[monitor_test.go](../../cmd/probe/monitor_test.go).
