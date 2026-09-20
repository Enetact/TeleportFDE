# Security model and review boundaries

This API changes Kubernetes workload capacity. It should run only in an isolated
lab until its full build, deployment behavior, and permissions have been reviewed.

## Implemented controls

TLS 1.3 only; server SAN verification; client chain, lifetime, and EKU verification;
an exact client URI SAN allowlist; no insecure API mode; separate non-data health
listener; bounded request size and deadlines; 0–1000 replica cap; input validation;
optimistic resourceVersion checks; UID checks against same-name replacements;
application-level protected targets; HPA conflict checks; non-root/read-only image;
resource-specific Kubernetes permissions; generated rather than embedded secrets.

Local CA key material remains on the developer's machine. The archive contains no
actual private keys, tokens, kubeconfigs, or pre-generated operational identities.
The negative-test certificate is created only when the developer runs certgen.

Integration verifies certificate rejection using `cmd/tlscheck`: fixtures must
load, be current and CA-trusted for client authentication, and have the intended
URI role. Only remote TLS certificate alerts pass a negative check. Each identity
uses an isolated port-forward; authenticated access is verified before and after
the negative checks. Missing files, connection refusal and timeouts fail closed.
CI retains selected diagnostic logs, excluding `.local/pki` and Secret manifests.

The rollout probe's completion endpoint listens only on loopback inside its Pod
and is reached with the existing KIND administrator's `kubectl exec` access.
It is not exposed by a Kubernetes Service and does not grant application access.

## Not implemented / not guaranteed

- Fine-grained authorization, tenant isolation, rate limits per identity, or
  prevention of all denial-of-service patterns. An authorized identity is a broad
  infrastructure operator, and Kubernetes credentials are correspondingly powerful.
- Certificate revocation, live certificate reload, production workload identity,
  automated trust-bundle overlap, or production CA/key custody. Leaf rotation
  requires a controlled rollout; a new CA requires a staged trust migration.
- Strict leader fencing, an atomic lock against HPA creation, or a transaction
  spanning intent and Deployment. Idempotency and conflict detection reduce risk;
  they do not create distributed transactions.
- A production audit trail, admission policy, network-policy enforcement, signed
  releases, SBOM, or a comprehensive vulnerability assessment. Reviewed tool,
  image and action pins and a passing Go vulnerability scan are present; they
  do not establish complete supply-chain security or scan every container OS package.
- Large-cluster API pagination, namespace-specific authorization, or a guarantee
  that an initially synchronized informer remains fully fresh during every failure.

The health port is omitted from the Service, but a cluster network peer may still
reach Pod IPs unless network policy/firewall rules prevent it. It returns only
status. Do not mistake omission from a Service for complete network isolation.

Private-key paths are ignored by Git. Never add `.local/` to a repository or share
its CA/client/server private keys. Never disable server certificate verification
to work around a DNS SAN mismatch; fix the identity/endpoint configuration.

## Before any non-lab use

Revalidate the selected dependency graph, Go vulnerabilities, image digests and
reviewed CI action pins against the intended release; replace development PKI;
reduce permission scope; add the required authorization and audit controls; verify
upgrade behavior under load and dependency outages; and have an independent review.
These are requirements for a deployment decision, not tasks claimed as completed
by this reference implementation.

The [integration report](INTEGRATION-VALIDATION.md) records local race/TLS tests,
fault-injection checks, the clean Go scan and live KIND results. It is development
evidence, not a security certification or approval for non-lab deployment.
