# Audit artifact checks

Executed 2026-09-20 for the new level guides and audit documentation.

| Check | Result |
|---|---|
| Existing reference manifest | 53/53 file hashes match the original manifest |
| Original integration/format shell syntax | PASS using Git Bash |
| Five new wrapper scripts | PASS Bash syntax |
| Wrapper routing from unrelated working directory | PASS for each level, explicit command and default help (10 invocations) |
| Wrapper routing after copying into a path containing spaces | PASS for each level (5 invocations) |
| Relative Markdown links in level guides/maps/index | 166 checked; zero broken |
| Optional Helm overlays | All five select the matching numeric level |
| Original tracked files | No edits made by this audit |

Wrapper routing was checked with a local fake `make` executable that validates
arguments and directory resolution. It did **not** run Go tests, build containers,
or deploy anything. The temporary check material is under ignored `.local/`.
The first Git Bash attempt failed because the inherited PATH did not contain its
Unix utilities; rerunning with Git Bash's `/usr/bin` and `/bin` supplied passed.
This was a validation-shell setup issue, not a wrapper application-test result.

No new Go test, race, vet, binary-build, Helm-render, Kubernetes-admission,
Docker-build or cluster-integration result is claimed. See [AUDIT.md](AUDIT.md).
The original `docs/VALIDATION.md` and its execution logs were preserved.
