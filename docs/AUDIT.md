# TeleportFDE audit

Follow-up: [dependency validation](DEPENDENCY-VALIDATION.md) records subsequent
toolchain, generated-source, formatting and workflow repairs. Findings below
describe the original baseline and are retained as historical evidence.

Audit date: 2026-09-20. Baseline: local commit `1d6cd0d` with a clean working tree
before this audit. Origin: `https://github.com/Enetact/TeleportFDE.git`.
Scope: public challenge alignment, local source and test inspection, folder
organization, reproducibility and relocation. This is not a passing cluster test
report, a security certification, or a hiring-level assessment.

## Verdict

**Correct public challenge; substantial source coverage; not submission-ready.**

The [official challenge index](https://github.com/gravitational/careers/blob/main/challenges/README.md)
points both Site Reliability Engineering and Forward Deployed Engineer to the
same [SRE challenge](https://github.com/gravitational/careers/blob/main/challenges/sre/challenge.md).
The requirements are cumulative assessment levels, not five mandatory repositories
or five required folder layouts. Confirm the assigned level and current process
with your panel before presenting this as an interview submission.

The supplied [job URL](https://jobs.ashbyhq.com/goteleport/219963ed-01b6-4e78-a9c8-7048c0109141)
returned the title “Site Reliability Engineer (Forward Deployed)” but its body
required JavaScript. Browser attachment timed out and the public job-feed request
failed TLS credential initialization. Consequently, the exact job responsibilities,
location and current availability were not verified. Challenge alignment is based
on the official careers index, not an inferred job description.

[gravitational/teleport](https://github.com/gravitational/teleport/tree/master)
is the actual infrastructure access product repository. This project is a separate
Kubernetes replica-control exercise. Its code does not integrate the Teleport
product; the exercise does not require copying or rebuilding that repository.
Teleport architecture and its [RFD format](https://github.com/gravitational/teleport/blob/master/rfd/0000-rfds.md)
are useful interview context. A separate product lab would be optional preparation.

## Level inventory

Before this audit there were **no level-1 through level-5 directories**. One server
selects behavior through `--level`; one Makefile selects it through `LEVEL`.
The requested [five level folders](../levels/README.md) now contain guides,
linked file maps, portable command wrappers and optional Helm overlays.
They share the original implementation and do not duplicate source.

| Level | Observed implementation | Main files relative to repository root | Assessment |
|---|---|---|---|
| [1](../levels/level-1/FILES.md) | HTTP replica GET; writes/list absent | `internal/httpapi`, `internal/service`, `internal/kube/backend.go`, `Dockerfile` | Source present; full build and deployment unverified |
| [2](../levels/level-2/FILES.md) | Adds PUT, input validation and conditional scaling | Above plus `internal/kube/backend_test.go`, `scripts/integration.sh` | Source/tests present; live results absent |
| [3](../levels/level-3/FILES.md) | Adds listing; background connectivity health; rolling Helm workload | `internal/health`, `charts/replica-control/templates`, HTTP list route | Source present; level-3 upgrade test excluded |
| [4](../levels/level-4/FILES.md) | Watch cache for HTTP reads; certificate and URI checks | `internal/kube/backend.go`, `internal/security`, `Makefile` | Cache test present; live cache/TLS/rollout evidence absent |
| [5](../levels/level-5/FILES.md) | gRPC; cached intent/Deployment reads; CRD, controller, leader election | `api/replicas/v1`, `internal/grpcapi`, `internal/kube/intents.go`, `internal/controller`, `internal/reconcile`, `cmd/replicactl` | Source present; generated code missing and runtime unverified |

All five profiles use mTLS and the shared chart. All server builds also compile
the level-5 adapter, even when the runtime level is 1. Switching to an earlier
level therefore does not bypass missing protobuf generation/dependencies.

## Findings, ordered by priority

### A1 — P1: The checked-in tree is not a complete reproducible build

`go.sum`, `gen/replicas/v1/replicas.pb.go` and
`gen/replicas/v1/replicas_grpc.pb.go` are absent. The Dockerfile copies the missing
lockfile at line 9, the server imports the missing generated package, and CI
explicitly requires all three files to be tracked at lines 28–29. A fresh direct
Docker build cannot complete as supplied; CI cannot pass its tracked-file gate.
This limitation is already disclosed in the original README.

Action: run `make prepare` with the required toolchain, review and commit the
actual dependency and generated outputs, then run full tests, vet and builds.
Do not substitute `go.offline.mod` for the application module.

The Go 1.26.8 pin is real: the [official release history](https://go.dev/doc/devel/release#go1.26)
lists it as released on September 1, 2026. This audit does not establish that every
module, image digest or generator pin resolves successfully. No dependency
upgrades are necessary merely to organize the folders.

### A2 — P1: The rollout test can miss an outage late in an upgrade

`scripts/integration.sh:167` starts a 75-second probe, then line 190 allows Helm
up to 180 seconds. Line 191 only checks that the Job eventually completed.
If an upgrade lasts longer than the probe, an outage after the probe finishes
will be invisible. A passing Job alone does not prove full-upgrade availability.

Action: synchronize probe lifetime with upgrade completion and a post-upgrade
observation period, keep timeout bounds for both processes, and retain timestamps
showing that measurement covered the entire upgrade. Also document the latency
criterion: `cmd/probe/main.go:57` uses `grpc.WaitForReady(true)` within a three-second
call deadline, so brief connection interruption may appear as latency rather
than a failed RPC. The current measurement is sampled, not a zero-interruption proof.

### A3 — P2: Level 3 has no supported upgrade-test command

`Makefile:88–90` accepts only levels 4 and 5; `.github/workflows/ci.yaml:59`
also excludes level 3. Level 3 already serves HTTP through the rolling chart,
but the automation cannot demonstrate its required upgrade availability.
The level guard even runs after deployment/build prerequisites.

Action: extend the supported rollout matrix to 3–5 and validate the requested
level before deploying. Measure the actual Service, retaining the existing
in-cluster probe approach rather than using a Pod-pinned port-forward as evidence.

### A4 — P2: A missing negative-test identity can be mistaken for mTLS rejection

`Makefile:60–64` reuses PKI when the CA and authorized client/server files exist;
it does not require the unauthorized client files. `scripts/integration.sh:96–98`
accepts any client-command failure as a successful authorization rejection.
Removing just the unauthorized certificate/key can therefore make this assertion
pass without testing a server handshake at all. Network/client failures can
produce the same misleading result.

Action: validate negative-test fixture presence, validity and intended identity;
check a successful authorized request on the same transport around the negative
case; distinguish local loading failures from server rejection. Retain the
existing dedicated TLS tests, which cover more cases than the integration script.

### A5 — P2: Important runtime wiring has no demonstrated failure/recovery coverage

`internal/controller` and `cmd/server` have no dedicated tests. The existing
reconciliation tests exercise decisions through a fake Store, not informer event
wiring, election or full process lifecycle. The explicit cached-read test uses
level 4, not the combined Deployment/intent caches at level 5. The cluster script
never explicitly exercises health degradation/recovery or leader termination.
The gRPC wire test uses an intentionally insecure in-memory transport; TLS tests
are separate. This is an evidence gap, not a claim that these components are broken.

Action: add focused checks for level-5 cache reads/watch updates, controller
restart and leader failover with durable intent, health failure/recovery,
concurrent writes, and Deployment deletion/recreation. Cover live gRPC mTLS and
preserve the existing happy/error tests. Avoid expanding into unrelated features.

## Useful implementation already present

- Transport-independent validation, bounded replica counts, and explicit zero
  versus missing replica requests.
- Scale-subresource writes with conflict handling; UID protection against scaling
  a same-name replacement; per-Deployment intent persistence with owner references.
- Cached reads without per-request Kubernetes GET/list at levels 4–5; separate
  background health checks; informer objects converted or copied before mutation.
- TLS 1.3, verified client chains, URI authorization, resource limits and non-root
  containers. The level-5 chart includes Role/RoleBinding for leases and
  ClusterRole/ClusterRoleBinding for its cluster-wide operations.
- Readiness-first shutdown and two-replica rolling settings. These are design
  mechanisms, not yet demonstrated availability outcomes.

Authorization remains coarse and cluster-wide; system-namespace, annotation and
HPA restrictions are application checks rather than a Kubernetes tenancy boundary.
Certificate rotation, multiple independent controller releases, stale-watch
behavior and scheduling capacity should be understood for the walkthrough;
several are already discussed in the design/security docs.

## Submission and interview preparation

The [challenge process](https://github.com/gravitational/careers/blob/main/challenges/sre/challenge.md)
asks for a Markdown design PR, two design approvals before implementation,
roughly 2–3 implementation PRs, and a walkthrough. It recommends writing your
own design and code and limiting scope to the target level. Its stated window
is two weeks after joining the panel's Slack channel.

The local README explicitly identifies this as an AI-generated educational
reference. Preserve that disclosure. Be able to explain, critique and reproduce
each design choice; confirm with the panel how prior preparation and AI assistance
fit their current process. This audit is suitable as edge-case review and learning
material. It does not establish personal authorship or reviewer approval.

Only two local commits were visible. Local history does not prove whether remote
PRs, panel feedback or approvals exist; those were not queried. Needed next:

1. Confirm target level/current instructions with the panel.
2. Resolve A1, then fix the rollout and negative-test evidence issues.
3. Run fresh full checks and the local cluster matrix, retaining actual results.
4. Prepare a walkthrough of API errors, mTLS, cache consistency, CRD conflicts,
   leader failure, shutdown and deployment tradeoffs.
5. If useful for the FDE role, practice explaining a failed customer deployment
   and recovery clearly; a separate Teleport access lab is optional, not a repair
   for an unmet replica-control requirement.

## Relocation and validation boundary

No machine-specific checkout paths were found in the tracked source/docs. Existing
`/tls`, `/src` and `/out` paths are container paths, not workstation locations.
Existing raw commands generally assume the repository is the current directory.
The added wrappers calculate `script_dir` and `repo_root`, quote them, and call
`make -C`. Their optional Helm overlays contain only the selected level.
The root README and original application/build/test files remain unchanged.

Checks performed during this audit:

- Read the local source, packaging, tests, CI and supplied validation records.
- Verified all 53 entries in the original `MANIFEST.json` against their SHA-256
  hashes before changes. No mismatch was found. It remains the original reference
  inventory, not an inventory of the newly added audit/level files.
- Confirmed the original shell scripts parse with Git Bash.
- Checked Windows and the existing Ubuntu WSL environment for required tools.
  No usable Go/KIND/Helm/protoc build workflow was exposed by those checks.
  WSL enumeration required execution outside the restricted sandbox; starting
  its shell also reported a systemd user-session warning.
- New guide/link/wrapper verification is recorded in
  [AUDIT-CHECKS.md](AUDIT-CHECKS.md).

Historical `docs/validation/core-tests.jsonl` remains historical evidence. No new
Go unit/race results, full build, Helm rendering, Docker build, cluster integration,
or deployment was produced by this audit. No software was installed, no cluster
was created, no code was committed, and nothing was sent to Teleport.
