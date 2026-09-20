#!/usr/bin/env bash
# This script targets only a named local KIND context and its own test namespace.
set -Eeuo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/toolchain.env"
source "$repo_root/scripts/port-forward.sh"
cd -- "$repo_root"
LEVEL=${LEVEL:-5}
CLUSTER=${CLUSTER:-sre-reference}
NAMESPACE=${NAMESPACE:-replica-system}
RELEASE=${RELEASE:-replica-control}
IMAGE=${IMAGE:-replica-control:dev}
PORT=${PORT:-18443}
CTX="kind-$CLUSTER"
TEST_NS="sre-it-$(date +%s)-$$"
PF_PID=""
K=(kubectl --context "$CTX")
mkdir -p .local
BODY=".local/$TEST_NS-response.json"
DIAGNOSTICS="$repo_root/artifacts/integration/level-$LEVEL/$TEST_NS"
mkdir -p "$DIAGNOSTICS"
PF_LOG=""
STAGE=setup
stage() { STAGE=$1; printf '%s stage=%s\n' "$(date -u +%FT%TZ)" "$STAGE" | tee -a "$DIAGNOSTICS/stages.log"; }
trap 'printf "Failed stage=%s line=%s status=%s\n" "$STAGE" "$LINENO" "$?" >&2' ERR
cleanup() {
  code=$?
  trap - EXIT ERR
  set +e
  if [[ "$code" != 0 ]]; then
    "${K[@]}" -n "$NAMESPACE" get pods -o wide >"$DIAGNOSTICS/pods.log" 2>&1 || true
    "${K[@]}" -n "$NAMESPACE" logs -l "app.kubernetes.io/instance=$RELEASE" --all-containers --prefix --tail=200 >"$DIAGNOSTICS/application.log" 2>&1 || true
    "${K[@]}" -n "$TEST_NS" get events --sort-by=.metadata.creationTimestamp >"$DIAGNOSTICS/test-events.log" 2>&1 || true
    "${K[@]}" -n "$NAMESPACE" get events --sort-by=.metadata.creationTimestamp >"$DIAGNOSTICS/application-events.log" 2>&1 || true
    "${K[@]}" -n "$TEST_NS" logs job/rollout-probe >"$DIAGNOSTICS/rollout.log" 2>&1 || true
    cat "$DIAGNOSTICS"/*.log >&2
  fi
  printf 'level=%s stage=%s exit=%s\n' "$LEVEL" "$STAGE" "$code" >"$DIAGNOSTICS/result.log"
  stop_forward
  "${K[@]}" delete namespace "$TEST_NS" --wait=false >/dev/null 2>&1 || true
  rm -f "$BODY"
  exit "$code"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
stage setup
"${K[@]}" create namespace "$TEST_NS" >/dev/null
"${K[@]}" -n "$TEST_NS" apply -f - <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  replicas: 1
  selector:
    matchLabels: {app: replica-test-demo}
  template:
    metadata:
      labels: {app: replica-test-demo}
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: pause
          image: "$PAUSE_IMAGE"
          resources:
            requests: {cpu: 1m, memory: 8Mi}
            limits: {cpu: 20m, memory: 32Mi}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: [ALL]}
YAML
"${K[@]}" -n "$TEST_NS" rollout status deployment/demo --timeout=90s
TLS=(--silent --show-error --max-time 5 --cacert .local/pki/ca.crt --cert .local/pki/client.crt --key .local/pki/client.key)
CLI=(.bin/replicactl --addr "127.0.0.1:$PORT")
URL="https://localhost:$PORT/v1/namespaces/$TEST_NS/deployments/demo/replicas"
get_replicas() {
  forward_alive || return 1
  if [[ "$LEVEL" == 5 ]]; then "${CLI[@]}" get "$TEST_NS" demo; else curl "${TLS[@]}" --fail "$URL"; fi
}
set_replicas() {
  forward_alive || return 1
  if [[ "$LEVEL" == 5 ]]; then "${CLI[@]}" set "$TEST_NS" demo "$1"; else curl "${TLS[@]}" --fail -X PUT -H 'Content-Type: application/json' -d "{\"replicas\":$1}" "$URL"; fi
}
wait_count() {
  local desired=$1
  for _ in {1..120}; do
    local actual
    actual=$("${K[@]}" -n "$TEST_NS" get deployment demo -o jsonpath='{.spec.replicas}')
    if [[ "$actual" == "$desired" ]]; then return 0; fi
    sleep 0.5
  done
  echo "Deployment did not reach desired replica spec $desired" >&2; return 1
}
wait_api_count() {
  local desired=$1
  for _ in {1..120}; do
    forward_alive || return 1
    if get_replicas >"$BODY" 2>/dev/null && jq -e --argjson n "$desired" '.replicas == $n' "$BODY" >/dev/null; then return 0; fi
    sleep 0.5
  done
  echo "API did not observe replica count $desired" >&2; return 1
}
# Each negative test first proves authenticated reachability on its own tunnel.
# Only an explicit remote certificate alert counts; EOF/reset/timeout do not.
for identity in missing unauthorized; do
  stage "tls-$identity"
  start_forward "$identity"
  wait_api_count 1
  if [[ "$identity" == missing ]]; then
    .bin/tlscheck --url "$URL" --no-cert 2>&1 | tee "$DIAGNOSTICS/tls-$identity.log"
  else
    .bin/tlscheck --url "$URL" --cert .local/pki/unauthorized-client.crt --key .local/pki/unauthorized-client.key 2>&1 | tee "$DIAGNOSTICS/tls-$identity.log"
  fi
  stop_forward
done
stage authenticated-api
start_forward authenticated
wait_api_count 1
if [[ "$LEVEL" -ge 3 ]]; then
  stage list
  if [[ "$LEVEL" == 5 ]]; then "${CLI[@]}" list >"$BODY"; else curl "${TLS[@]}" --fail "https://localhost:$PORT/v1/deployments" >"$BODY"; fi
  jq -e --arg ns "$TEST_NS" 'any(.deployments[]; .namespace == $ns and .name == "demo")' "$BODY" >/dev/null
fi
if [[ "$LEVEL" -ge 2 ]]; then
  stage scaling-and-validation
  set_replicas 3 >"$BODY"
  if [[ "$LEVEL" == 5 ]]; then jq -e '.accepted == true' "$BODY" >/dev/null; fi
  wait_count 3; wait_api_count 3
  if [[ "$LEVEL" == 5 ]]; then
    if "${CLI[@]}" set "$TEST_NS" demo -1 >"$BODY" 2>&1; then echo 'Negative replicas accepted' >&2; exit 1; fi
    grep -q 'InvalidArgument' "$BODY"
    if .bin/replicactl --addr "localhost:$PORT" --expected-version stale set "$TEST_NS" demo 4 >"$BODY" 2>&1; then echo 'Stale version accepted' >&2; exit 1; fi
    grep -q 'Aborted' "$BODY"
    "${K[@]}" -n "$TEST_NS" scale deployment/demo --replicas=1
    wait_count 3; wait_api_count 3
    "${K[@]}" -n "$TEST_NS" rollout status deployment/demo --timeout=90s
    "${K[@]}" -n "$TEST_NS" wait --for=jsonpath='{.status.phase}'=Ready replicaintent/demo --timeout=90s
    "${K[@]}" -n "$TEST_NS" get replicaintent demo -o json | jq -e '.status.observedGeneration == .metadata.generation' >/dev/null
  else
    code=$(curl "${TLS[@]}" -o "$BODY" -w '%{http_code}' -X PUT -H 'Content-Type: application/json' -d '{"replicas":-1}' "$URL")
    [[ "$code" == 400 ]]
    code=$(curl "${TLS[@]}" -o "$BODY" -w '%{http_code}' -X PUT -H 'Content-Type: application/json' -d '{"replicas":4,"expectedVersion":"stale"}' "$URL")
    [[ "$code" == 409 ]]
  fi
  # The API must not take replica ownership from an existing HPA.
  stage hpa-conflict
  "${K[@]}" -n "$TEST_NS" autoscale deployment demo --min=1 --max=4 --cpu-percent=70
  if [[ "$LEVEL" == 5 ]]; then
    if "${CLI[@]}" set "$TEST_NS" demo 2 >"$BODY" 2>&1; then echo 'HPA conflict accepted' >&2; exit 1; fi
    grep -q 'FailedPrecondition' "$BODY"
  else
    code=$(curl "${TLS[@]}" -o "$BODY" -w '%{http_code}' -X PUT -H 'Content-Type: application/json' -d '{"replicas":2}' "$URL")
    [[ "$code" == 412 ]]
  fi
  "${K[@]}" -n "$TEST_NS" delete hpa demo
  set_replicas 3 >"$BODY"; wait_count 3; wait_api_count 3
fi
if [[ "${UPGRADE_TEST:-0}" == 1 ]]; then
  stage rollout
  "${K[@]}" -n "$TEST_NS" create secret generic probe-tls --from-file=client.crt=.local/pki/client.crt --from-file=client.key=.local/pki/client.key --from-file=ca.crt=.local/pki/ca.crt
  "${K[@]}" -n "$TEST_NS" apply -f - <<YAML
apiVersion: batch/v1
kind: Job
metadata:
  name: rollout-probe
spec:
  backoffLimit: 0
  activeDeadlineSeconds: 360
  template:
    spec:
      restartPolicy: Never
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: probe
          image: "$IMAGE"
          imagePullPolicy: IfNotPresent
          command: [/probe]
          args:
            - --level=$LEVEL
            - --addr=$RELEASE.$NAMESPACE.svc:8443
            - --server-name=$RELEASE.$NAMESPACE.svc
            - --namespace=$TEST_NS
            - --name=demo
            - --duration=300s
            - --controlled
            - --settle=10s
          resources:
            requests: {cpu: 25m, memory: 32Mi}
            limits: {cpu: 250m, memory: 128Mi}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: [ALL]}
          volumeMounts:
            - name: tls
              mountPath: /tls
              readOnly: true
      volumes:
        - name: tls
          secret:
            secretName: probe-tls
            defaultMode: 0440
YAML
  for _ in {1..120}; do
    if "${K[@]}" -n "$TEST_NS" logs job/rollout-probe 2>/dev/null | grep -q 'probe started'; then break; fi
    sleep 0.5
  done
  "${K[@]}" -n "$TEST_NS" logs job/rollout-probe | grep -q 'probe started'
  helm upgrade "$RELEASE" charts/replica-control --kube-context "$CTX" -n "$NAMESPACE" --reuse-values --set-string rolloutToken="$(date +%s)-$$" --wait=watcher --rollback-on-failure --timeout 180s
  # The running probe must acknowledge completion; an early exit fails exec.
  "${K[@]}" -n "$TEST_NS" exec job/rollout-probe -- /probe --finish
  "${K[@]}" -n "$TEST_NS" wait --for=condition=complete job/rollout-probe --timeout=60s
  "${K[@]}" -n "$TEST_NS" logs job/rollout-probe | tee "$DIAGNOSTICS/rollout.log"
  start_forward after-rollout
  wait_api_count 3
fi
stage final-authenticated-check
get_replicas >"$BODY"
if [[ "$LEVEL" -ge 2 ]]; then set_replicas 0 >"$BODY"; wait_count 0; wait_api_count 0; fi
stage complete
printf 'Level %s integration checks passed.\n' "$LEVEL"
