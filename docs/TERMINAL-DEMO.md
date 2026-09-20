# Demonstrate every level in terminals

**Author and system architect:** Jamie Holland.

Use the existing scripts for repeatable assertions and a separate manual session
to explain API behavior. Complete [development setup](DEVELOPMENT.md) first and
start Docker. All Bash examples run in Ubuntu/WSL from the repository root.
Paths derive from `repo_root`; the lab cluster is named `sre-demo`.

Run levels sequentially. A deployment selects one level for the application
release; the shared checkout also reuses build outputs, image tags and test port
18443. Five tabs running different levels against that same release would
interfere with one another.

## 1. Automated tests on screen

From PowerShell at the repository root, choose one level:

```powershell
$repoRoot = (Get-Location).Path
$level = 2
& (Join-Path $repoRoot 'scripts/dev.ps1') -MakeArguments @(
    'integration', "LEVEL=$level", 'CLUSTER=sre-demo'
)
```

Change `$level` to 1–5. For levels 3–5, replace `integration` with `upgrade-test`
to include the same integration checks plus the live rollout probe. That target
already runs the integration checks, so a separate integration run is unnecessary
for a demo that needs both.

For an entire narrated demonstration, this WSL block runs each level in order
and retains terminal output. It stops at the first failure and preserves the
test's failure status even though output is also sent to `tee`:

```bash
repo_root="$(pwd)"
(
  set -euo pipefail
  run_id="$(date -u +%Y%m%dT%H%M%SZ)"
  log_dir="$repo_root/artifacts/terminal-demo/$run_id"
  mkdir -p "$log_dir"
  for level in 1 2 3 4 5; do
    target=integration
    if (( level >= 3 )); then target=upgrade-test; fi
    printf '\n=== LEVEL %s: %s ===\n' "$level" "$target"
    bash "$repo_root/scripts/dev.sh" "$target" "LEVEL=$level" CLUSTER=sre-demo \
      2>&1 | tee "$log_dir/level-$level.log"
  done
)
```

The source-only checks can be shown once because all levels share the code:

```bash
bash "$repo_root/scripts/dev.sh" quality vuln
```

The harness prints stages such as `tls-missing`, `authenticated-api`, `list`,
`scaling-and-validation`, `hpa-conflict`, `rollout` and `complete`, as applicable.
Its final success message is `Level N integration checks passed.` A stage marker
alone is not proof that the stage succeeded. Read the process exit status and
`artifacts/integration/level-N/<test-namespace>/result.log` as well.

| Level | What the automated run demonstrates |
|---|---|
| 1 | Authenticated HTTP read; missing and unauthorized certificate rejection; normal access afterward |
| 2 | Level-1 checks plus scale-up, zero, invalid count, stale version and HPA conflict |
| 3 | Level-2 checks plus listing; `upgrade-test` measures requests through a rolling deployment |
| 4 | The same external HTTP contract with cached reads; `upgrade-test` measures rollout behavior |
| 5 | gRPC read/set/list, persisted intent, drift correction, readiness and conflicts; `upgrade-test` also measures rollout behavior |

After the cluster starts, a second WSL terminal can watch Deployment counts:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
kubectl --context kind-sre-demo get deployments --all-namespaces --watch
```

Optional third terminal for application lifecycle/controller logs:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
kubectl --context kind-sre-demo -n replica-system logs \
  -l app.kubernetes.io/instance=replica-control \
  --all-containers=true --prefix=true --follow=true --max-log-requests=10
```

Start the log command after Pods are running and restart it after a rollout to
follow replacement Pods. The service does not log a full audit event for every
API request. Harness output, API responses and Kubernetes watches provide the
other views. Ctrl+C stops a watch or log follower without deleting the cluster.

The harness removes its disposable target namespace on completion but leaves
the application running. Create the separate fixture below for manual API calls.

## 2. Prepare an interactive demo

Finish automated tests first. In terminal A at the repository root, select a
level and create a disposable target. Use an unused namespace name if
`sre-demo-manual` already exists; use that same name in the other terminals.

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
level=2
demo_ns=sre-demo-manual
bash "$repo_root/scripts/dev.sh" deploy build "LEVEL=$level" CLUSTER=sre-demo
kubectl --context kind-sre-demo create namespace "$demo_ns"
kubectl --context kind-sre-demo -n "$demo_ns" create deployment demo \
  --image="$PAUSE_IMAGE" --replicas=1
kubectl --context kind-sre-demo -n "$demo_ns" rollout status deployment/demo --timeout=90s
kubectl --context kind-sre-demo -n replica-system \
  port-forward --address=127.0.0.1 service/replica-control 19443:8443
```

Leave the forwarding command running. Port 19443 is reserved here for the manual
demo; the automated harness uses 18443. A forward selects one Pod, so restart it
after changing levels or replacing Pods. It is not a rollout availability probe.

Terminal B displays the actual target replica counts:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
demo_ns=sre-demo-manual
kubectl --context kind-sre-demo -n "$demo_ns" get deployment demo --watch
```

Terminal C sends requests. Initialize these variables once in that terminal:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
demo_ns=sre-demo-manual
base_url=https://localhost:19443
replicas_url="$base_url/v1/namespaces/$demo_ns/deployments/demo/replicas"
http=(curl --silent --show-error --max-time 10 \
  --cacert "$repo_root/.local/pki/ca.crt" \
  --cert "$repo_root/.local/pki/client.crt" \
  --key "$repo_root/.local/pki/client.key")
grpc=("$repo_root/.bin/replicactl" --addr localhost:19443 --server-name localhost \
  --ca "$repo_root/.local/pki/ca.crt" \
  --cert "$repo_root/.local/pki/client.crt" --key "$repo_root/.local/pki/client.key")
```

The arrays keep certificate and path options consistent without disabling TLS
verification. No token, external account or Teleport cluster is involved.

## 3. API catalog

| Level | Public operations | Read source / write behavior |
|---|---|---|
| 1 | HTTP GET replicas | Live Kubernetes read |
| 2 | HTTP GET/PUT replicas | Live read and direct scale write |
| 3 | HTTP GET/PUT replicas; GET Deployment list | Live read, direct write, listing |
| 4 | Same HTTP operations as level 3 | Informer-cached reads; direct write |
| 5 | gRPC `GetReplicas`, `SetReplicas`, `ListDeployments`; health Check | Cached reads; Set saves intent for the elected controller |

The full REST routes are:

- `GET /v1/namespaces/{namespace}/deployments/{name}/replicas`
- `PUT /v1/namespaces/{namespace}/deployments/{name}/replicas` (levels 2–4)
- `GET /v1/deployments?namespace={namespace}` (levels 3–4; omit the query for all namespaces)

Level 5 replaces those routes with `replicas.v1.ReplicaService` RPCs. The
[protobuf contract](../api/replicas/v1/replicas.proto) and
[CLI](../cmd/replicactl/main.go) define the request and response fields. There is
no workload-creation API; the manual fixture is created with Kubernetes tooling.

### Level 1: show the read contract

```bash
"${http[@]}" --fail "$replicas_url" | jq .
```

Explain `replicas` as the Deployment's desired count, `readyReplicas` as ready
Pods and `availableReplicas` as available Pods. Their values need not be equal
during scheduling or a rollout. The response also identifies namespace, name,
UID and resource version. PUT and listing are not exposed at level 1.

### Level 2: scale and show rejected requests

```bash
"${http[@]}" --fail -X PUT -H 'Content-Type: application/json' \
  -d '{"replicas":3}' "$replicas_url" | jq .
"${http[@]}" --fail "$replicas_url" | jq .

# Expected HTTP 400: an invalid replica count.
"${http[@]}" -i -X PUT -H 'Content-Type: application/json' \
  -d '{"replicas":-1}' "$replicas_url"

# Expected HTTP 409: a version that cannot match the current object.
"${http[@]}" -i -X PUT -H 'Content-Type: application/json' \
  -d '{"replicas":4,"expectedVersion":"stale"}' "$replicas_url"

# Intentional scale-to-zero is valid.
"${http[@]}" --fail -X PUT -H 'Content-Type: application/json' \
  -d '{"replicas":0}' "$replicas_url" | jq .
```

Use `-i` without `--fail` for expected rejections so the audience can see the
status and JSON error. Those demonstration requests do not assert correctness;
the automated harness does. Missing `replicas`, unknown fields and counts above
1000 are also invalid. A real conditional update uses `resourceVersion` from the
latest GET as `expectedVersion`; a concurrent Kubernetes update can still cause
a conflict.

### Level 3: list and demonstrate rollout

Use the level-2 calls, then list the namespace or entire cluster:

```bash
"${http[@]}" --fail "$base_url/v1/deployments?namespace=$demo_ns" | jq .
"${http[@]}" --fail "$base_url/v1/deployments" | jq .
```

Show rolling availability with `upgrade-test LEVEL=3` from section 1. The probe
runs inside Kubernetes against the real Service. Its summary reports request
counts, failures and timing, and the harness checks a post-upgrade observation
period. Do not use a manual port-forward's survival as evidence of availability.

### Level 4: explain cached reads

The calls are identical to level 3. Change replicas directly through Kubernetes,
then read the API to observe convergence:

```bash
kubectl --context kind-sre-demo -n "$demo_ns" scale deployment/demo --replicas=2
"${http[@]}" --fail "$replicas_url" | jq .
```

An immediate read can show the old count; repeat it after the watch catches up.
This illustrates eventual consistency, but it does not independently prove that
the request made no Kubernetes call. For that design claim, show the cached read
path in [internal/kube/backend.go](../internal/kube/backend.go). Level 4 does not
persist an intent that restores the earlier count.

### Level 5: show gRPC, persisted intent and drift correction

Deploy level 5 and restart terminal A's forward before using these calls. CLI
flags must precede the command. The first Set is unconditional:

```bash
"${grpc[@]}" health
"${grpc[@]}" list "$demo_ns"
"${grpc[@]}" get "$demo_ns" demo
"${grpc[@]}" set "$demo_ns" demo 3
kubectl --context kind-sre-demo -n "$demo_ns" wait \
  --for=jsonpath='{.status.phase}'=Ready replicaintent/demo --timeout=90s
"${grpc[@]}" get "$demo_ns" demo
kubectl --context kind-sre-demo -n "$demo_ns" get replicaintent demo -o yaml
```

The first Set creates the intent. If reusing an existing intent, a previous
`Ready` phase can remain briefly after a new write. Check that
`status.observedGeneration` matches `metadata.generation`, that spec/status
counts match the request, and that the Deployment has caught up. An immediate
cached GET is not an atomic snapshot of both resources.

In an extra terminal, watch intent phase and the generation it has observed:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
demo_ns=sre-demo-manual
kubectl --context kind-sre-demo -n "$demo_ns" get replicaintent demo --watch \
  -o 'custom-columns=NAME:.metadata.name,DESIRED:.spec.replicas,GEN:.metadata.generation,SEEN:.status.observedGeneration,PHASE:.status.phase,READY:.status.readyReplicas'
```

Back in terminal C, deliberately introduce drift on this disposable target:

```bash
kubectl --context kind-sre-demo -n "$demo_ns" scale deployment/demo --replicas=1
"${grpc[@]}" get "$demo_ns" demo
```

The controller should restore the Deployment spec to 3 because its saved intent
still requests 3. The change can be too fast to catch with one GET; the watches
and the harness's bounded assertions provide better evidence. Show invalid input
and a stale version as well:

```bash
"${grpc[@]}" set "$demo_ns" demo -1
"${grpc[@]}" --expected-version stale set "$demo_ns" demo 4
```

These commands are expected to exit nonzero with `InvalidArgument` and `Aborted`.
For a genuine conditional update, use `intentVersion` from the GET, not its
Deployment `resourceVersion`. Intent status writes can change that version too.
`accepted: true` means persisted intent, not ready Pods. The health command's
JSON must report `SERVING`; a successful health RPC can return `NOT_SERVING`
without a nonzero CLI exit code.

## 4. Health endpoints and useful evidence

All levels also have a separate Pod health listener on port 8081:

| Route | Meaning |
|---|---|
| `GET /livez` | Process responds; independent of Kubernetes availability |
| `GET /readyz` | Initial cache synchronization where applicable, recent successful dependency check, and not draining |
| `GET /healthz` | Alias of readiness |

That listener is not exposed by the application Service. To show it locally,
forward the Deployment's Pod health port in another terminal:

```bash
repo_root="$(pwd)"
source "$repo_root/scripts/dev-env.sh"
kubectl --context kind-sre-demo -n replica-system \
  port-forward --address=127.0.0.1 deployment/replica-control 19081:8081
```

From terminal C:

```bash
curl --silent --show-error --max-time 5 -i http://127.0.0.1:19081/livez
curl --silent --show-error --max-time 5 -i http://127.0.0.1:19081/readyz
```

For failures, show `result.log`, the relevant stage log, Kubernetes events and
application logs. Missing/unauthorized client tests should use the existing
harness: a TLS rejection can terminate a port-forward, and connection refusal
must not be presented as proof that authentication worked.

## 5. Switch levels and clean up

Stop manual forwards before changing levels. Delete only the disposable manual
namespace you created, then deploy the next level and recreate the fixture.
This also removes any level-5 intent so it cannot resume unexpectedly later:

```bash
kubectl --context kind-sre-demo delete namespace "$demo_ns" --wait=true
```

After the demonstration, stop watches/forwards and delete only the named lab:

```bash
bash "$repo_root/scripts/dev.sh" clean-cluster CLUSTER=sre-demo
```

Logs in `artifacts/` and local PKI are retained. These instructions were checked
against the scripts, routes and CLI and their code blocks syntax-checked; writing
this guide does not constitute a new live cluster test. See the dated
[integration evidence](INTEGRATION-VALIDATION.md) and
[CI controls](CI-CONTROLS.md) for previous runs and GitHub selection behavior.
