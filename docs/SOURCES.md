# Source references

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
- KIND v0.31.0 release and Kubernetes 1.35.0 node digest used in Makefile:
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

The exact transitive dependency graph is determined by the real `make prepare`
run and should be reviewed and committed. This document is not a substitute for
`go.sum`, an image digest, or a record of executed cluster tests.
