# Audit artifact checks

> Historical audit-artifact checks. The manifest match and counts below refer to the original audit snapshot, not the updated checkout. Current application and cluster results are in [integration validation](INTEGRATION-VALIDATION.md).

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

## Documentation reconciliation after integration repairs

Checked on 2026-09-20 against the current scripts, workflow, module/tool pins and
`docs/validation/integration-summary.json`. This follow-up updates documentation;
the results above remain the original audit snapshot.

| Check | Result |
|---|---|
| Repository Markdown inventory | 28 files reviewed for current versus historical scope |
| Local Markdown link targets | 314 references checked; zero missing targets |
| Bash examples | 34 blocks parsed with Git Bash; no commands executed |
| PowerShell examples | Nine blocks parsed with the PowerShell parser; no commands executed |
| Current command names | Reviewed against Makefile; removed the imported unsupported `fuzz-core` instruction |
| Application/build provenance | All hashes recorded in the integration receipt still match |
| Offline HTML guides | All six pages passed local-link, layout and browser checks |
| Visual behavior | Playback, GIF motion, reduced-motion defaults and zero external requests passed |
| Whitespace | Git diff check passed |

The root validation index, level evidence sections, file maps, walkthrough,
design/security guides and dependency report now describe the completed local
checks. Historical imported review claims and original tool versions are labeled
as such; missing review-specific receipts are not promoted to verified evidence.
The integration report also states the HTTP probe's response-body limitation.
Remote GitHub execution remains distinct from local Linux/ARM64 evidence.

Application tests were not rerun for this documentation-only pass. Prior test
results remain tied to the unchanged code hashes. Upstream links were not
revalidated; this pass checked local documentation consistency and link targets.
