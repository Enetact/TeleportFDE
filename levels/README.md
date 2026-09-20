# Level guides

[Repository setup](../README.md) | [Animated level map](../docs/visuals/levels.html) | [Current build results](../docs/DEPENDENCY-VALIDATION.md)

These five folders organize the shared Go implementation by runtime level.
The official careers index maps both SRE and Forward Deployed Engineer to the
[same public challenge](https://github.com/gravitational/careers/blob/main/challenges/README.md).
Separate folders are a navigation choice for this repository, not an official
submission requirement. Agree the assessed level with the interview panel.

| Folder | Shared behavior to study |
|---|---|
| [level-1](level-1/README.md) | HTTP replica reads |
| [level-2](level-2/README.md) | HTTP replica writes |
| [level-3](level-3/README.md) | Listing, health and rolling deployment |
| [level-4](level-4/README.md) | Informer cache; mTLS is already shared by all levels |
| [level-5](level-5/README.md) | gRPC, ReplicaIntent and reconciliation |

Each contains a guide, linked file map, portable Bash command wrapper and optional
Helm level overlay. Source, tests, dependencies, chart and CI remain at the root.
The wrappers locate that root from their own directory. No checkout location is
hardcoded. Invoke the wrappers with `bash` rather than relying on executable file bits.

After the one-time [development setup](../docs/DEVELOPMENT.md), start at the
repository root. For example:

```bash
repo_root="$(pwd)"
bash "$repo_root/levels/level-1/run.sh" quality vuln
bash "$repo_root/levels/level-1/run.sh" integration
```

The second command creates or updates the named local KIND lab and requires
Docker. Replace `level-1` with the chosen folder; `upgrade-test` is available for
levels 3–5. Each level README includes the PowerShell/WSL equivalent. Generated
bindings are supplied; ordinary checks do not require `prepare` first.

The [original audit](../docs/AUDIT.md) records baseline findings. Subsequent
toolchain, generated-source and workflow fixes are described in the
[current validation report](../docs/DEPENDENCY-VALIDATION.md); live cluster
acceptance is still separate from passing shared-source tests. `MANIFEST.json`
records the original imported reference files, not the current guide inventory.
