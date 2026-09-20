# Source references

Current checkout authorities are [toolchain.env](../toolchain.env),
[go.mod](../go.mod) and [go.sum](../go.sum). They select Go 1.27.1,
client-go v0.37.0, gRPC v1.83.2, protobuf Go v1.36.12, KIND v0.33.0,
Kubernetes v1.37.0 and Helm v4.3.0. The
[dependency report](DEPENDENCY-VALIDATION.md) explains the security backport and
pins; [integration validation](INTEGRATION-VALIDATION.md) records executed tests.
The version-specific references below belong to the original authoring baseline,
not the current package selections. Upstream websites were not rechecked during
this documentation-only reconciliation.

Checked during preparation on September 20, 2026. These are primary references,
not endorsements. No upstream challenge implementation has been copied. The
GitHub-rendered challenge page was used for the requirements; the raw-file web
cache returned an older wording and was not treated as the latest authority.

- Teleport SRE challenge and candidate guidance:
  https://github.com/gravitational/careers/blob/main/challenges/sre/challenge.md
- client-go informers and cache documentation:
  https://pkg.go.dev/k8s.io/client-go/tools/cache
- client-go leader election, including its no-strict-fencing limitation:
  https://pkg.go.dev/k8s.io/client-go/tools/leaderelection
- Kubernetes CustomResourceDefinitions, validation, and status subresources:
  https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
- Deployment rollout behavior:
  https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
- Go TLS configuration and certificate verification:
  https://pkg.go.dev/crypto/tls
- client-go v0.35.0 module baseline (Go 1.25 minimum):
  https://github.com/kubernetes/client-go/blob/v0.35.0/go.mod
- gRPC-Go v1.78.0 module baseline:
  https://github.com/grpc/grpc-go/blob/v1.78.0/go.mod
- Protocol Buffers Go v1.36.11 module baseline:
  https://github.com/protocolbuffers/protobuf-go/blob/v1.36.11/go.mod
- Historical KIND v0.31.0 release and Kubernetes 1.35.0 node baseline:
  https://github.com/kubernetes-sigs/kind/releases/tag/v0.31.0
- Helm 3.19.0 baseline:
  https://github.com/helm/helm/releases/tag/v3.19.0
- Go release downloads:
  https://go.dev/dl/

Official installation references for the developer's machine:

- https://go.dev/doc/install
- https://docs.docker.com/get-started/get-docker/
- https://kubernetes.io/docs/tasks/tools/
- https://kind.sigs.k8s.io/docs/user/quick-start/
- https://helm.sh/docs/intro/install/
- https://protobuf.dev/installation/

The selected graph and generated outputs are supplied. `make prepare` explicitly
updates those inputs after a deliberate schema/dependency change; review and
commit its outputs together. This document is not a substitute for
`go.sum`, an image digest, or a record of executed cluster tests.
