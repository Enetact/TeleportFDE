// Editorial content is kept separate from rendering so the guide can evolve with the code.
export const guides = [
  {
    slug: 'architecture', number: '01', title: 'One core. Five levels.',
    subtitle: 'How a request moves through Replica Control', category: 'SYSTEM ARCHITECTURE',
    summary: 'Follow an authenticated request from the operator to shared Go services, informer caches and Kubernetes.',
    principle: 'Kubernetes owns the state. The service controls access.',
    caption: 'Read paths change by level; one shared application implements them all.',
    facts: [['Identity first', 'TLS 1.3 + client URI'], ['Shared core', 'HTTP or gRPC adapter'], ['State source', 'Kubernetes API']],
    notes: [
      ['Authenticate before serving', 'The operational API on port 8443 requires a trusted client certificate and the exact allowed URI SAN. The Kubernetes Service routes traffic; the application terminates TLS.'],
      ['Pick the read path', 'Levels 1–3 read live Kubernetes data. Levels 4–5 serve API reads from a cache in each Pod. Background list/watch traffic keeps those caches current.'],
      ['Keep storage in Kubernetes', 'Levels 2–4 write the Deployment scale subresource. Level 5 writes a ReplicaIntent, then an elected controller converges scale. There is no external database.']
    ],
    caveat: 'Caches are eventually consistent. A successful write does not guarantee the next cached read has observed it.',
    refs: ['cmd/server/main.go', 'internal/service/service.go', 'internal/kube/backend.go', 'internal/security/tls.go']
  },
  {
    slug: 'levels', number: '02', title: 'Five levels. One journey.',
    subtitle: 'The challenge grows through cumulative capabilities', category: 'CHALLENGE MAP',
    summary: 'See what each level adds, which transport it uses, and how the shared source stays connected.',
    principle: 'These are capability steps, not five running services.',
    caption: 'The level selector chooses a profile within the same compiled server.',
    facts: [['Levels 1–3', 'Live HTTP reads'], ['Level 4', 'Cached HTTP reads'], ['Level 5', 'gRPC + durable intent']],
    notes: [
      ['Levels 1 and 2 · the API foundation', 'Level 1 reads replica counts. Level 2 adds controlled scale writes, input validation, conflict handling and ownership checks.'],
      ['Levels 3 and 4 · operational behavior', 'Level 3 adds Deployment listing; the shared chart and health system support operation through rolling changes. Level 4 moves reads to informer caches. This reference uses mTLS at every level.'],
      ['Level 5 · asynchronous control', 'gRPC replaces the HTTP operational API. SetReplicas persists intent; controller workers apply it. Every level folder points to the same shared source and build.']
    ],
    caveat: 'The folders are study profiles. A diagram or source inventory does not establish that a challenge level passed live acceptance.',
    refs: ['levels/README.md', 'levels/level-1/FILES.md', 'levels/level-2/FILES.md', 'levels/level-3/FILES.md', 'levels/level-4/FILES.md', 'levels/level-5/FILES.md']
  },
  {
    slug: 'reconciliation', number: '03', title: 'Intent becomes state.',
    subtitle: 'Inside the level-5 reconciliation loop', category: 'CONTROLLER ARCHITECTURE',
    summary: 'Separate request acceptance from convergence, and see why the controller checks ownership and versions again.',
    principle: 'Accepted means persisted. Ready comes later.',
    caption: 'Illustrated happy path and blocked branch; retries are not timed telemetry.',
    facts: [['One leader', 'Two queue workers'], ['Safe target', 'UID + version checks'], ['Recovery', 'Events + 30s requeue']],
    notes: [
      ['Persist the desired outcome', 'SetReplicas checks the live Deployment and writes a same-name ReplicaIntent tied to the immutable Deployment UID. The response returns acceptance and an intent resource version.'],
      ['Recheck before changing scale', 'Only the elected leader runs workers. Reconciliation checks deletion, UID, protection, HPA ownership and resource versions, then writes the scale subresource.'],
      ['Observe and converge', 'Informer events enqueue keys. Successful passes periodically requeue existing intent after 30 seconds; failures use bounded retries and backoff. Status records Ready, Reconciling or Blocked.']
    ],
    caveat: 'There is no cross-resource transaction or strict leader fencing. Version-aware, idempotent reconciliation reduces races; competing replica controllers remain unsupported.',
    refs: ['internal/kube/intents.go', 'internal/controller/controller.go', 'internal/reconcile/reconcile.go', 'cmd/server/main.go']
  },
  {
    slug: 'rollout', number: '04', title: 'Make room. Then drain.',
    subtitle: 'A rolling update designed to keep serving', category: 'AVAILABILITY DESIGN',
    summary: 'Trace the lifecycle from a new Pod to readiness, Service routing and bounded shutdown of the old Pod.',
    principle: 'Readiness controls routing. Liveness stays independent.',
    caption: 'A design sequence, not proof of uninterrupted availability in a live cluster.',
    facts: [['Rolling policy', '0 unavailable / 1 surge'], ['Stability gate', '5s minReadySeconds'], ['Termination', '40s Pod grace period']],
    notes: [
      ['Start capacity before retiring it', 'The chart defaults to two API Pods, maxUnavailable=0 and maxSurge=1. Spare scheduling capacity and working networking are still required.'],
      ['Wait until a Pod can serve', 'Readiness requires initial cache sync where applicable and a recent Kubernetes connectivity check. A ready Pod can receive Service traffic. minReadySeconds adds a five-second stability period before Deployment counts it available for rollout progress; it does not delay Service routing.'],
      ['Stop accepting, then finish work', 'On shutdown, the server marks readiness false, waits five seconds for endpoint propagation, and allows up to fifteen seconds for request draining before forced stop.'],
      ['Observe the complete upgrade', 'An in-cluster probe first proves authenticated access. It samples throughout Helm and continues for ten seconds after the harness acknowledges completion. Missing completion or an early exit fails the check.']
    ],
    caveat: 'The controlled probe has a 300-second deadline and samples with a three-second request limit. Passing is bounded sampled evidence, not a guarantee of zero latency. Live results are recorded separately.',
    refs: ['charts/replica-control/templates/deployment.yaml', 'charts/replica-control/templates/service.yaml', 'internal/health/health.go', 'cmd/server/main.go', 'scripts/integration.sh', 'cmd/probe/monitor.go', 'docs/INTEGRATION-VALIDATION.md']
  },
  {
    slug: 'delivery', number: '05', title: 'Source to a local image.',
    subtitle: 'The development checks that travel with the repository', category: 'BUILD & DELIVERY',
    summary: 'Connect the local toolchain to GitHub checks, container builds, automatic PR and push cluster tests, and version-tag artifacts.',
    principle: 'The same pinned requirements power local checks and CI.',
    caption: 'Version tags create artifacts; the workflow does not deploy to a remote cluster.',
    facts: [['Source gate', 'Format + tests + scan'], ['Runtime', 'Static, non-root image'], ['Cluster checks', 'PRs + pushes']],
    notes: [
      ['Review build inputs', 'toolchain.env maps tools and image pins. go.mod and go.sum select and verify packages. Generated protobuf bindings are committed and compared against fresh generation.'],
      ['Keep checks reproducible', 'The format inventory includes tracked and nonignored untracked Go files, including generated code. The quality gate also runs race tests, vet, four host builds, tunnel lifecycle regressions, chart rendering and workflow validation. The TLS rejection helper runs on the host; the scratch runtime retains three binaries.'],
      ['Exercise every PR and push', 'After quality passes, PRs and pushes run the KIND matrix for all five levels and rollout probes for levels 3–5. Certificate checks use isolated tunnels. Per-level diagnostic logs are retained for seven days before cluster cleanup; private keys are excluded. A push to an open PR can run both matrices. Manual runs can enable the tests; version tags also package the chart and image archive.']
    ],
    caveat: 'All five local KIND levels and rollout checks for levels 3–5 passed on Linux/ARM64. GitHub-hosted results for the fixes are still separate evidence. Moving particles illustrate the workflow.',
    refs: ['toolchain.env', 'Makefile', 'Dockerfile', '.github/workflows/ci.yaml', 'scripts/port-forward.sh', 'cmd/tlscheck/main.go', 'docs/DEPENDENCY-VALIDATION.md', 'docs/INTEGRATION-VALIDATION.md']
  }
];
