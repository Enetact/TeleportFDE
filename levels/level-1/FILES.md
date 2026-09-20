# Level 1 file map

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
| [.github/workflows/ci.yaml](../../.github/workflows/ci.yaml) | Unit checks and opt-in cluster matrix |
| [scripts/integration.sh](../../scripts/integration.sh) | Level-selected live checks |
| [scripts/check-format.sh](../../scripts/check-format.sh) | Go formatting check |
| [docs/DESIGN.md](../../docs/DESIGN.md) | Existing educational design and complete protobuf contract |
| [docs/VALIDATION.md](../../docs/VALIDATION.md) | Historical validation boundary |
| [docs/SECURITY.md](../../docs/SECURITY.md) | Security assumptions and limitations |

## Level implementation and tests

- [internal/httpapi/http.go](../../internal/httpapi/http.go)
- [internal/httpapi/http_test.go](../../internal/httpapi/http_test.go)
- [internal/model/model_test.go](../../internal/model/model_test.go)
- [internal/service/service_test.go](../../internal/service/service_test.go)
- [internal/security/tls_test.go](../../internal/security/tls_test.go)
- [internal/health/health_test.go](../../internal/health/health_test.go)

