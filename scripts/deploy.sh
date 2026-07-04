#!/usr/bin/env bash
set -euo pipefail

# Bring up the full Phasma stack on a local Kubernetes cluster (e.g. colima/k3s).
# Idempotent: safe to re-run; reuses the cluster, namespace, and port-forward.

CLUSTER="${CLUSTER:-phasma}"
NS="${NS:-phasma}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
K8S_DIR="$ROOT/deploy"
REGISTRY="${REGISTRY:-localhost:5000/phasma}"
GIT_SHA="${GIT_SHA:-$(git -C "${ROOT}" rev-parse --short HEAD)}"
APP_HOST="${APP_HOST:-phasma.localhost}"
LOCAL_PORT="${LOCAL_PORT:-8080}"
REMOTE_PORT="${REMOTE_PORT:-8080}"
LOCAL_ORIGIN="${LOCAL_ORIGIN:-http://${APP_HOST}:${LOCAL_PORT}}"
PORT_FORWARD_LOG="${PORT_FORWARD_LOG:-/tmp/phasma-port-forward-${LOCAL_PORT}.log}"
PORT_FORWARD_PID_FILE="${PORT_FORWARD_PID_FILE:-/tmp/phasma-port-forward-${LOCAL_PORT}.pid}"
STORAGE_IMAGE="chrislusf/seaweedfs:3.76"
CACHE_IMAGE="docker.dragonflydb.io/dragonflydb/dragonfly:v1.25.0"
SEARCH_IMAGE="getmeili/meilisearch:v1.11.3"

SERVICES=(backend database frontend)
ROLL_OUT_DATABASE=(statefulset/database)
ROLL_OUT_REST=(
  statefulset/storage
  statefulset/cache
  statefulset/search
  statefulset/broker
  deployment/backend
  deployment/frontend
)
ROLL_OUT_CONNECT=(deployment/connect)

log() {
  echo "==> $*"
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_tools() {
  local tool
  for tool in kubectl docker make; do
    command -v "$tool" >/dev/null || die "missing required tool: $tool"
  done
}

require_docker() {
  docker info >/dev/null 2>&1 || die "Docker daemon is not running. Start Docker and try again."
}

require_colima_aio_capacity() {
  local context aio_nr aio_max available
  context="$(kubectl config current-context 2>/dev/null || true)"
  [[ "${context}" == "colima" ]] || return 0
  command -v colima >/dev/null || return 0

  aio_nr="$(colima ssh -- cat /proc/sys/fs/aio-nr 2>/dev/null || true)"
  aio_max="$(colima ssh -- cat /proc/sys/fs/aio-max-nr 2>/dev/null || true)"
  [[ "${aio_nr}" =~ ^[0-9]+$ && "${aio_max}" =~ ^[0-9]+$ ]] || return 0
  available=$((aio_max - aio_nr))

  if (( aio_max >= 1048576 && available >= 4096 )); then
    return 0
  fi

  cat >&2 <<EOF
error: Colima Linux AIO capacity is too low for Redpanda.

Current Colima values:
  fs.aio-nr      ${aio_nr}
  fs.aio-max-nr  ${aio_max}
  available      ${available}

Raise the node limit, then rerun deploy:
  colima ssh -- sudo sysctl -w fs.aio-max-nr=1048576

To persist it in the Colima VM:
  colima ssh -- sudo sh -c 'echo fs.aio-max-nr=1048576 >/etc/sysctl.d/99-redpanda-aio.conf && sysctl --system'
EOF
  exit 1
}

random_secret() {
  if command -v openssl >/dev/null; then
    openssl rand -hex 32
    return
  fi

  local secret
  secret="$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 64 || true)"
  [[ ${#secret} -eq 64 ]] || die "failed to generate a secret"
  printf '%s\n' "${secret}"
}

ensure_namespace() {
  kubectl create namespace "${NS}" 2>/dev/null || true
}

ensure_secret() {
  # database-secret
  if ! kubectl -n "${NS}" get secret database-secret >/dev/null 2>&1; then
    kubectl -n "${NS}" create secret generic database-secret \
      --from-literal=database-password="$(random_secret)"
  else
    ensure_secret_key database-secret database-password
  fi
  # backend-secret
  if ! kubectl -n "${NS}" get secret backend-secret >/dev/null 2>&1; then
    kubectl -n "${NS}" create secret generic backend-secret \
      --from-literal=session-hash-secret="$(random_secret)"
  else
    ensure_secret_key backend-secret session-hash-secret
  fi
  # storage-secret
  if ! kubectl -n "${NS}" get secret storage-secret >/dev/null 2>&1; then
    kubectl -n "${NS}" create secret generic storage-secret \
      --from-literal=s3-access-key="$(random_secret)" \
      --from-literal=s3-secret-key="$(random_secret)"
  else
    ensure_secret_key storage-secret s3-access-key
    ensure_secret_key storage-secret s3-secret-key
  fi
  # cache-secret
  if ! kubectl -n "${NS}" get secret cache-secret >/dev/null 2>&1; then
    kubectl -n "${NS}" create secret generic cache-secret \
      --from-literal=cache-password="$(random_secret)"
  else
    ensure_secret_key cache-secret cache-password
  fi
  # search-secret
  if ! kubectl -n "${NS}" get secret search-secret >/dev/null 2>&1; then
    kubectl -n "${NS}" create secret generic search-secret \
      --from-literal=search-master-key="$(random_secret)"
  else
    ensure_secret_key search-secret search-master-key
  fi
}

ensure_app_db_secret() {
  if ! kubectl -n "${NS}" get secret app-db-secret >/dev/null 2>&1; then
    kubectl -n "${NS}" create secret generic app-db-secret \
      --from-literal=app-db-password="$(random_secret)"
  fi
}

ensure_connect_secret() {
  if ! kubectl -n "${NS}" get secret connect-secret >/dev/null 2>&1; then
    # search-connect-key starts empty; provision_meili_connect_key fills it after Meilisearch is ready.
    kubectl -n "${NS}" create secret generic connect-secret \
      --from-literal=connect-db-password="$(random_secret)" \
      --from-literal=search-connect-key=""
  else
    ensure_secret_key connect-secret connect-db-password
  fi
}

provision_meili_connect_key() {
  local existing
  existing="$(kubectl -n "${NS}" get secret connect-secret -o go-template="{{ index .data \"search-connect-key\" }}" 2>/dev/null || true)"
  if [[ -n "${existing}" && "${existing}" != "<no value>" && "${existing}" != "$(printf '' | base64)" ]]; then
    return
  fi

  log "provisioning Meilisearch connect key"
  local master_key pf_pid key encoded
  master_key="$(kubectl -n "${NS}" get secret search-secret -o go-template="{{ index .data \"search-master-key\" }}" | base64 -d)"

  kubectl -n "${NS}" port-forward service/search 7701:7700 >/dev/null 2>&1 &
  pf_pid=$!
  trap 'kill "${pf_pid}" 2>/dev/null || true' RETURN

  local i=0
  until curl -sf "http://localhost:7701/health" -H "Authorization: Bearer ${master_key}" >/dev/null 2>&1; do
    i=$((i + 1))
    [[ $i -lt 20 ]] || { kill "${pf_pid}" 2>/dev/null || true; die "Meilisearch did not become ready in time"; }
    sleep 1
  done

  key="$(curl -sf -X POST "http://localhost:7701/keys" \
    -H "Authorization: Bearer ${master_key}" \
    -H "Content-Type: application/json" \
    -d "{\"actions\":[\"documents.add\",\"documents.delete\"],\"indexes\":[\"users\",\"posts\",\"hashtags\"],\"expiresAt\":null,\"description\":\"phasma-connect scoped key\"}" \
    | grep -o '"key":"[^"]*"' | cut -d'"' -f4)"
  [[ -n "${key}" ]] || die "failed to provision Meilisearch connect key"

  encoded="$(printf '%s' "${key}" | base64 | tr -d '\n')"
  kubectl -n "${NS}" patch secret connect-secret --type merge \
    -p "{\"data\":{\"search-connect-key\":\"${encoded}\"}}" >/dev/null
  kill "${pf_pid}" 2>/dev/null || true
  trap - RETURN
}

ensure_secret_key() {
  local secret_name="$1"
  local key="$2"
  local existing encoded
  existing="$(kubectl -n "${NS}" get secret "${secret_name}" -o go-template="{{ index .data \"${key}\" }}")"
  if [[ -n "${existing}" && "${existing}" != "<no value>" ]]; then
    return
  fi

  log "adding missing ${key} secret"
  encoded="$(printf '%s' "$(random_secret)" | base64 | tr -d '\n')"
  kubectl -n "${NS}" patch secret "${secret_name}" --type merge \
    -p "{\"data\":{\"${key}\":\"${encoded}\"}}" >/dev/null
}

ensure_tls_secret() {
  local secret_name="frontend-tls"
  local tmpdir
  if kubectl -n "${NS}" get secret "${secret_name}" >/dev/null 2>&1; then
    return
  fi
  command -v openssl >/dev/null || die "missing required tool for local TLS secret: openssl"

  log "creating self-signed TLS secret for ${APP_HOST}"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' RETURN
  openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
    -keyout "${tmpdir}/tls.key" \
    -out "${tmpdir}/tls.crt" \
    -subj "/CN=${APP_HOST}" \
    -addext "subjectAltName=DNS:${APP_HOST}" >/dev/null 2>&1
  kubectl -n "${NS}" create secret tls "${secret_name}" \
    --cert="${tmpdir}/tls.crt" \
    --key="${tmpdir}/tls.key" >/dev/null
  trap - RETURN
  rm -rf "${tmpdir}"
}

ensure_database_tls_secret() {
  local secret_name="database-tls"
  local tmpdir
  if kubectl -n "${NS}" get secret "${secret_name}" >/dev/null 2>&1; then
    return
  fi
  command -v openssl >/dev/null || die "missing required tool for database TLS secret: openssl"

  log "creating self-signed TLS secret for database"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' RETURN
  openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
    -keyout "${tmpdir}/tls.key" \
    -out "${tmpdir}/tls.crt" \
    -subj "/CN=database" \
    -addext "subjectAltName=DNS:database,DNS:database.phasma.svc.cluster.local" >/dev/null 2>&1
  kubectl -n "${NS}" create secret tls "${secret_name}" \
    --cert="${tmpdir}/tls.crt" \
    --key="${tmpdir}/tls.key" >/dev/null
  trap - RETURN
  rm -rf "${tmpdir}"
}

port_pids() {
  if command -v lsof >/dev/null; then
    lsof -nP -iTCP:"${LOCAL_PORT}" -sTCP:LISTEN -t 2>/dev/null || true
  fi
}

is_frontend_port_forward() {
  local pid="$1"
  local command
  command="$(ps -p "${pid}" -o command= 2>/dev/null || true)"
  [[ "${command}" == *"kubectl"* ]] &&
    [[ "${command}" == *"port-forward"* ]] &&
    [[ "${command}" == *"service/frontend"* ]] &&
    [[ "${command}" == *"${LOCAL_PORT}:${REMOTE_PORT}"* ]] &&
    [[ "${command}" == *"${NS}"* ]]
}

handle_existing_port_forward() {
  local pids pid
  pids="$(port_pids)"
  if [[ -z "${pids}" ]]; then
    return 1
  fi

  while IFS= read -r pid; do
    if is_frontend_port_forward "${pid}"; then
      echo "Frontend port-forward is already running on ${LOCAL_ORIGIN}/ (pid ${pid})."
      return 0
    fi
  done <<< "${pids}"

  echo "error: local port ${LOCAL_PORT} is already in use by another process:" >&2
  while IFS= read -r pid; do
    ps -p "${pid}" -o pid=,command= >&2 || true
  done <<< "${pids}"
  echo "Stop that process or rerun with a different port, for example:" >&2
  echo "  LOCAL_PORT=8081 $0" >&2
  exit 1
}

build_images() {
  log "building images"
  export DOCKER_BUILDKIT=1
  export GIT_SHA
  make -C "${ROOT}" GIT_SHA="${GIT_SHA}"
  if docker container inspect --format '{{.State.Running}}' phasma-control-plane 2>/dev/null | grep -qx true; then
    log "loading images into kind node"
    docker pull "${STORAGE_IMAGE}"
    docker pull "${CACHE_IMAGE}"
    docker pull "${SEARCH_IMAGE}"
    docker save "${REGISTRY}/backend:${GIT_SHA}" | docker exec -i phasma-control-plane ctr --namespace k8s.io images import -
    docker save "${REGISTRY}/database:${GIT_SHA}" | docker exec -i phasma-control-plane ctr --namespace k8s.io images import -
    docker save "${REGISTRY}/frontend:${GIT_SHA}" | docker exec -i phasma-control-plane ctr --namespace k8s.io images import -
    docker save "${STORAGE_IMAGE}" | docker exec -i phasma-control-plane ctr --namespace k8s.io images import -
    docker save "${CACHE_IMAGE}" | docker exec -i phasma-control-plane ctr --namespace k8s.io images import -
    docker save "${SEARCH_IMAGE}" | docker exec -i phasma-control-plane ctr --namespace k8s.io images import -
  fi
}

apply_manifests() {
  log "creating namespace and applying manifests"
  ensure_namespace
  ensure_secret
  ensure_app_db_secret
  ensure_connect_secret
  ensure_tls_secret
  ensure_database_tls_secret
  kubectl -n "${NS}" delete job broker-backfill --ignore-not-found --wait=true >/dev/null
  kubectl apply -f "${K8S_DIR}" -n "${NS}"
  kubectl -n "${NS}" set env deployment/frontend ORIGIN="${LOCAL_ORIGIN}" >/dev/null
  kubectl -n "${NS}" set image deployment/backend \
    migration="${REGISTRY}/database:${GIT_SHA}" \
    backend="${REGISTRY}/backend:${GIT_SHA}" >/dev/null
  kubectl -n "${NS}" set image deployment/frontend frontend="${REGISTRY}/frontend:${GIT_SHA}" >/dev/null
}

rollout_restart() {
  local resource
  for resource in "$@"; do
    kubectl -n "${NS}" rollout restart "${resource}"
  done
}

wait_for_rollouts() {
  local failed=()
  local resource
  for resource in "$@"; do
    if ! kubectl -n "${NS}" rollout status "${resource}" --timeout=180s; then
      failed+=("${resource}")
    fi
  done

  if [[ ${#failed[@]} -eq 0 ]]; then
    return 0
  fi

  echo "error: rollout failed for: ${failed[*]}" >&2
  echo "==> current pod statuses"
  kubectl -n "${NS}" get pods
  for resource in "${failed[@]}"; do
    echo "==> recent logs for ${resource}"
    case "${resource}" in
      statefulset/broker)
        kubectl -n "${NS}" logs statefulset/broker -c broker --tail=40 || true
        ;;
      deployment/connect)
        kubectl -n "${NS}" logs deployment/connect -c connect --tail=40 || true
        ;;
      *)
        kubectl -n "${NS}" logs "${resource}" --tail=40 || true
        ;;
    esac
  done
  exit 1
}

run_broker_backfill() {
  log "running broker backfill"
  kubectl -n "${NS}" delete job broker-backfill --ignore-not-found --wait=true >/dev/null
  kubectl apply -f "${K8S_DIR}/broker.yaml" -n "${NS}" >/dev/null
  kubectl -n "${NS}" patch job broker-backfill --type merge -p '{"spec":{"suspend":false}}' >/dev/null
  if kubectl -n "${NS}" wait --for=condition=complete job/broker-backfill --timeout=180s; then
    return 0
  fi

  echo "error: broker backfill failed" >&2
  kubectl -n "${NS}" get pods -l job-name=broker-backfill
  kubectl -n "${NS}" logs job/broker-backfill --tail=80 || true
  kubectl -n "${NS}" logs job/broker-backfill -c provision-connect-user --tail=80 || true
  exit 1
}

restart_stack() {
  log "restarting all services"
  # Restarting them together ensures the backends drop their DB connections,
  # allowing the database's graceful shutdown to complete instantly.
  rollout_restart "${ROLL_OUT_DATABASE[@]}" "${ROLL_OUT_REST[@]}"

  log "waiting for database"
  wait_for_rollouts "${ROLL_OUT_DATABASE[@]}"

  log "waiting for application services"
  wait_for_rollouts "${ROLL_OUT_REST[@]}"

  provision_meili_connect_key

  log "restarting connect (after broker is ready)"
  rollout_restart "${ROLL_OUT_CONNECT[@]}"
  wait_for_rollouts "${ROLL_OUT_CONNECT[@]}"

  run_broker_backfill
}

start_port_forward_background() {
  local supervisor_pid

  # Terminate any existing frontend port-forward to this port to avoid stale connections
  local pids pid
  pids="$(port_pids)"
  if [[ -n "${pids}" ]]; then
    while IFS= read -r pid; do
      if is_frontend_port_forward "${pid}"; then
        log "stopping existing frontend port-forward (pid ${pid})"
        kill "${pid}" 2>/dev/null || true
      fi
    done <<< "${pids}"
    sleep 1
  fi

  if handle_existing_port_forward; then
    return 0
  fi

  log "starting frontend port-forward in the background"
  LOCAL_PORT="${LOCAL_PORT}" REMOTE_PORT="${REMOTE_PORT}" NS="${NS}" \
    nohup bash -c '
    set -u
    while true; do
      kubectl -n "${NS}" port-forward service/frontend "${LOCAL_PORT}:${REMOTE_PORT}"
      status=$?
      echo "port-forward exited with status ${status}; reconnecting in 3 seconds" >&2
      sleep 3
    done
  ' >> "${PORT_FORWARD_LOG}" 2>&1 &

  supervisor_pid=$!
  disown "${supervisor_pid}" 2>/dev/null || true
  echo "${supervisor_pid}" > "${PORT_FORWARD_PID_FILE}"

  sleep 2
  if handle_existing_port_forward; then
    echo "Background port-forward supervisor pid: ${supervisor_pid}"
    return 0
  fi

  if ps -p "${supervisor_pid}" >/dev/null 2>&1; then
    echo "Background port-forward is starting on ${LOCAL_ORIGIN}/ (supervisor pid ${supervisor_pid})."
    return 0
  fi

  echo "error: failed to start background port-forward. Recent logs:" >&2
  tail -30 "${PORT_FORWARD_LOG}" >&2 || true
  exit 1
}

print_summary() {
  cat <<EOF

==> phasma is up

  Frontend       ${LOCAL_ORIGIN}
  Gateway        in-cluster: http://backend:8080
  Namespace      ${NS}
  Context        $(kubectl config current-context 2>/dev/null || echo "unknown")

  Port-forward   supervisor pid: $(cat "${PORT_FORWARD_PID_FILE}" 2>/dev/null || echo "unknown")
                 logs: ${PORT_FORWARD_LOG}
                 stop: kill \$(cat ${PORT_FORWARD_PID_FILE})

  Pods           kubectl -n ${NS} get pods
  App logs       kubectl -n ${NS} logs deployment/<service> --tail=100
  Database logs  kubectl -n ${NS} logs statefulset/database --tail=100
  Tear down      kubectl delete namespace ${NS}

EOF
}

require_tools
require_docker
require_colima_aio_capacity

if [[ -n "$(port_pids)" ]]; then
  echo "note: local port ${LOCAL_PORT} is already in use; deploy will reuse a frontend port-forward or report the conflict." >&2
fi

build_images
apply_manifests
restart_stack
start_port_forward_background
print_summary
