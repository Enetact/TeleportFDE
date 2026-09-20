# Level 1: HTTP replica reads

[All levels](../README.md) | [File map](FILES.md) | [Audit](../../docs/AUDIT.md)

## Behavior in this repository

The HTTP GET route reads Deployment replicas directly from Kubernetes. PUT and list routes are not registered.

This folder uses the shared source at the repository root. It is a study/run
profile, not a separate codebase or evidence that this level has passed.
The same server binary compiles every adapter, including gRPC, at all levels.
All levels require the full bootstrap for a server build and use mTLS in this reference.

## Folder structure

```text
README.md     Level behavior, commands and evidence needed
FILES.md      Links to shared implementation, tests and packaging
run.sh        Resolves the repository directory and forwards a make command
values.yaml   Optional Helm values overlay selecting this level
```

## Commands

Use a Linux or macOS shell with the tools in the [root README](../../README.md).
Start in the repository root:

```bash
repo_root="$(pwd)"
bash "$repo_root/levels/level-1/run.sh" test-core
bash "$repo_root/levels/level-1/run.sh" doctor
bash "$repo_root/levels/level-1/run.sh" prepare
bash "$repo_root/levels/level-1/run.sh" test vet build helm-check
bash "$repo_root/levels/level-1/run.sh" integration
```

`integration` builds/deploys to a named local KIND cluster and modifies disposable
Kubernetes resources. It leaves the application release running. Choose a lab
cluster name before generating certificates; see the root README for overrides.
The wrapper forwards additional make arguments and fixes LEVEL=1.
It resolves paths from its own location, so it also works from another directory.

The values overlay can be used when rendering Helm directly:

```bash
helm template replica-control "$repo_root/charts/replica-control" \
  --namespace replica-system --include-crds \
  --values "$repo_root/levels/level-1/values.yaml"
```

This renders manifests only. Deployment still needs the documented image, TLS
Secret and CRD preparation. The shared make workflow selects the same level via
LEVEL; it does not consume this optional values overlay.

## Evidence still needed

Demonstrate an existing Deployment, a missing target, disabled writes, and a working container deployment.

Keep command output, tool versions, source commit and cluster details with the
result. Historical core test logs do not establish that this level works end to
end. See [AUDIT.md](../../docs/AUDIT.md) for current findings and validation limits.
