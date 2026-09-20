# Level 2: HTTP replica writes

[All levels](../README.md) | [File map](FILES.md) | [Visual level map](../../docs/visuals/levels.html) | [Build results](../../docs/DEPENDENCY-VALIDATION.md)

## Behavior in this repository

Adds PUT requests, replica validation, scale-subresource writes, version checks, and HPA/protected-target guards. Reads still use Kubernetes directly.

This folder uses the shared source at the repository root. It is a study/run
profile, not a separate codebase or evidence that this level has passed.
The same server binary compiles every adapter, including gRPC, at all levels.
All levels need the shared Go toolchain and dependencies and use mTLS in this
reference. The generated bindings and dependency checksums are already included.

## Folder structure

```text
README.md     Level behavior, commands and evidence needed
FILES.md      Links to shared implementation, tests and packaging
run.sh        Resolves the repository directory and forwards a make command
values.yaml   Optional Helm values overlay selecting this level
```

## Commands

Complete the [development setup](../../docs/DEVELOPMENT.md) once. In an
Ubuntu/WSL terminal (or a manually configured macOS shell), start in the repository
root, not this level folder:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
bash "$repo_root/levels/level-2/run.sh" quality vuln
```

For local cluster checks, with Docker running:

```bash
bash "$repo_root/levels/level-2/run.sh" doctor
bash "$repo_root/levels/level-2/run.sh" integration
```

From PowerShell at the repository root, use the WSL wrapper:

```powershell
$repoRoot = (Get-Location).Path
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments @('quality', 'vuln', 'LEVEL=2')
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments @('integration', 'LEVEL=2')
```

`quality` and `vuln` validate the shared implementation, not just one level.
Use `format` to repair Go formatting. Run `prepare` only after intentionally
changing the schema, dependency or generator selection; then rerun the checks.

`integration` builds/deploys to a named local KIND cluster and modifies disposable
Kubernetes resources. It leaves the application release running. Choose a lab
cluster name before generating certificates; see the root README for overrides.
The wrapper forwards additional make arguments and fixes LEVEL=2.
It resolves paths from its own location, so it also works from another directory.

The values overlay can be used when rendering Helm directly:

```bash
helm template replica-control "$repo_root/charts/replica-control" \
  --namespace replica-system --include-crds \
  --values "$repo_root/levels/level-2/values.yaml"
```

This renders manifests only. Deployment still needs the documented image, TLS
Secret and CRD preparation. The shared make workflow selects the same level via
LEVEL; it does not consume this optional values overlay.

## Evidence still needed

Demonstrate scale-up, zero replicas, invalid input, stale-version rejection, and HPA conflict handling against KIND.

Keep command output, tool versions, source commit and cluster details with the
result. Shared source tests, image builds and scans passed in the
[latest recorded local validation](../../docs/DEPENDENCY-VALIDATION.md); live level
acceptance remains unverified. The [original audit](../../docs/AUDIT.md) preserves
the baseline findings. The [visual guides](../../docs/visuals/index.html) explain
the implementation without claiming live test results.
