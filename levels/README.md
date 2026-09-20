# Level guides

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
| [level-4](level-4/README.md) | Informer cache and mTLS |
| [level-5](level-5/README.md) | gRPC, ReplicaIntent and reconciliation |

Each contains a guide, linked file map, portable Bash command wrapper and optional
Helm level overlay. Source, tests, dependencies, chart and CI remain at the root.
The wrappers locate that root from their own directory. No checkout location is
hardcoded. Use `bash level-N/run.sh` rather than relying on executable file bits.

These folders do not change existing behavior or repair the findings in
[the audit](../docs/AUDIT.md). `MANIFEST.json` records the original imported
reference files; it does not inventory these newly added guides.
