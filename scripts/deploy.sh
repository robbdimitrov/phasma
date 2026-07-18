#!/usr/bin/env bash
set -euo pipefail

# Bring up the local Phasma stack idempotently on a Kubernetes cluster.

NS="${NS:-phasma}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
K8S_DIR="$ROOT/deploy"
REGISTRY="${REGISTRY:-localhost:5000/phasma}"
APP_HOST="${APP_HOST:-phasma.localhost}"
LOCAL_PORT="${LOCAL_PORT:-8080}"
REMOTE_PORT="${REMOTE_PORT:-8080}"
LOCAL_ORIGIN="${LOCAL_ORIGIN:-http://${APP_HOST}:${LOCAL_PORT}}"
PORT_FORWARD_LOG="${PORT_FORWARD_LOG:-/tmp/phasma-port-forward-${LOCAL_PORT}.log}"
PORT_FORWARD_PID_FILE="${PORT_FORWARD_PID_FILE:-/tmp/phasma-port-forward-${LOCAL_PORT}.pid}"
BACKEND_IMAGE_TAG="${BACKEND_IMAGE_TAG:-}"
DATABASE_IMAGE_TAG="${DATABASE_IMAGE_TAG:-}"
FRONTEND_IMAGE_TAG="${FRONTEND_IMAGE_TAG:-}"
FORCE_BACKFILL="${FORCE_BACKFILL:-0}"
FORCE_BUILD="${FORCE_BUILD:-0}"

# storage/database are hard startup deps for backend; stage+wait avoids
# racing an unhealthy one, and unchanged stages stay no-ops.
ROLL_OUT_INFRA=(
  statefulset/storage
  statefulset/cache
  statefulset/search
  statefulset/broker
)
ROLL_OUT_DATABASE=(statefulset/database)
ROLL_OUT_REST=(
  deployment/backend
  deployment/frontend
)
ROLL_OUT_CONNECT=(deployment/connect)

INFRA_MANIFESTS=(
  "storage.yaml"
  "cache.yaml"
  "search.yaml"
)
DATABASE_MANIFESTS=("database.yaml")
BROKER_MANIFEST="broker.yaml"
STATIC_MANIFESTS=(
  "serviceaccounts.yaml"
  "networkpolicy.yaml"
  "pdb.yaml"
)

log() {
  echo "==> $*"
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_tools() {
  local tool
  for tool in kubectl docker make curl openssl; do
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

context_checksum() {
  local dir="$1"
  (
    cd "${ROOT}/${dir}"
    find . -type f \
      ! -path './.git/*' \
      ! -path './bin/*' \
      ! -path './tmp/*' \
      ! -path './coverage/*' \
      ! -path './node_modules/*' \
      ! -path './.svelte-kit/*' \
      ! -path './build/*' \
      ! -path './dist/*' \
      -print |
      LC_ALL=C sort |
      while IFS= read -r file; do
        case "${dir}:${file}" in
          apps/backend:./api | apps/backend:./api/*) continue ;;
          apps/backend:./AGENTS.md | apps/backend:./CLAUDE.md | apps/backend:./README.md) continue ;;
          apps/backend:*_test.go) continue ;;
          apps/frontend:*.md | apps/frontend:./.env | apps/frontend:./.env.*)
            [[ "${file}" == "./.env.example" ]] || continue
            ;;
          apps/frontend:./AGENTS.md | apps/frontend:./CLAUDE.md) continue ;;
          apps/frontend:*.test.ts) continue ;;
        esac
        printf '%s\0' "${file}"
        openssl dgst -sha256 -binary "${file}"
      done
  ) | openssl dgst -sha256 -r | awk '{print substr($1, 1, 12)}'
}

init_image_tags() {
  BACKEND_IMAGE_TAG="${BACKEND_IMAGE_TAG:-$(context_checksum apps/backend)}"
  DATABASE_IMAGE_TAG="${DATABASE_IMAGE_TAG:-$(context_checksum apps/database)}"
  FRONTEND_IMAGE_TAG="${FRONTEND_IMAGE_TAG:-$(context_checksum apps/frontend)}"
}

image_exists() {
  docker manifest inspect "$1" >/dev/null 2>&1
}

build_image() {
  local target="$1"
  local tag="$2"
  local image="${REGISTRY}/${target}:${tag}"
  if [[ "${FORCE_BUILD}" != "1" ]] && image_exists "${image}"; then
    log "skipping ${target}; image already exists: ${image}"
    return 0
  fi
  make -C "${ROOT}" "${target}" IMAGE_PREFIX="${REGISTRY}" GIT_SHA="${tag}"
}

build_images() {
  local pids=()
  local failed=0
  log "building images"
  log "image tags: backend=${BACKEND_IMAGE_TAG} database=${DATABASE_IMAGE_TAG} frontend=${FRONTEND_IMAGE_TAG}"
  export DOCKER_BUILDKIT=1
  build_image backend "${BACKEND_IMAGE_TAG}" &
  pids+=("$!")
  build_image database "${DATABASE_IMAGE_TAG}" &
  pids+=("$!")
  build_image frontend "${FRONTEND_IMAGE_TAG}" &
  pids+=("$!")
  for pid in "${pids[@]}"; do
    wait "${pid}" || failed=1
  done
  (( failed == 0 )) || die "image build failed"
}

apply_manifest_files() {
  local file files=()
  for file in "$@"; do
    files+=(-f "${K8S_DIR}/${file}")
  done
  kubectl apply "${files[@]}" -n "${NS}" >/dev/null
}

select_manifest_documents() {
  local manifest="$1"
  local mode="$2"
  local resource_kind="$3"
  local name="$4"
  awk -v mode="${mode}" -v want_kind="${resource_kind}" -v want_name="${name}" '
    function reset() {
      doc = ""
      doc_kind = ""
      doc_name = ""
      in_metadata = 0
    }
    function should_emit() {
      matched = (doc_kind == want_kind && doc_name == want_name)
      return mode == "only" ? matched : !matched
    }
    function emit() {
      if (doc != "" && should_emit()) {
        printf "%s---\n", doc
      }
    }
    /^---[[:space:]]*$/ {
      emit()
      reset()
      next
    }
    {
      doc = doc $0 "\n"
    }
    /^kind:[[:space:]]*/ {
      doc_kind = $2
    }
    /^metadata:[[:space:]]*$/ {
      in_metadata = 1
      next
    }
    /^[^[:space:]]/ && $0 !~ /^metadata:/ {
      in_metadata = 0
    }
    in_metadata && /^[[:space:]]+name:[[:space:]]*/ {
      doc_name = $2
    }
    END {
      emit()
    }
  ' "${K8S_DIR}/${manifest}"
}

apply_app_manifest() {
  local manifest="$1"
  local name="$2"
  local deployment
  deployment="$(mktemp)"
  shift 2

  select_manifest_documents "${manifest}" only Service "${name}" |
    kubectl apply -f - -n "${NS}" >/dev/null
  select_manifest_documents "${manifest}" only Deployment "${name}" > "${deployment}"
  if ! kubectl set image --local -o yaml -f "${deployment}" "$@" |
    kubectl apply -f - -n "${NS}" >/dev/null; then
    rm -f "${deployment}"
    return 1
  fi
  rm -f "${deployment}"
}

apply_app_manifests() {
  apply_app_manifest "backend.yaml" backend \
    migration="${REGISTRY}/database:${DATABASE_IMAGE_TAG}" \
    backend="${REGISTRY}/backend:${BACKEND_IMAGE_TAG}"

  apply_app_manifest "frontend.yaml" frontend \
    frontend="${REGISTRY}/frontend:${FRONTEND_IMAGE_TAG}"
}

apply_broker_infra_manifest() {
  local rendered
  rendered="$(mktemp)"
  trap 'rm -f "${rendered}"' RETURN
  select_manifest_documents "${BROKER_MANIFEST}" except Job broker-backfill > "${rendered}"
  kubectl apply -f "${rendered}" -n "${NS}" >/dev/null
  trap - RETURN
  rm -f "${rendered}"
}

data_resource_checksum() {
  local resource_kind="$1"
  local name="$2"
  # shellcheck disable=SC2016 # go-template variables must reach kubectl literally.
  kubectl -n "${NS}" get "${resource_kind}" "${name}" -o go-template='{{ range $k, $v := .data }}{{ printf "%s=%s\n" $k $v }}{{ end }}' \
    | LC_ALL=C sort \
    | openssl dgst -sha256 -r | awk '{print $1}'
}

annotate_data_resource_checksums() {
  local resource_kind="$1"
  local resource="$2"
  shift 2
  local name pairs=()
  for name in "$@"; do
    pairs+=("\"checksum/${name}\":\"$(data_resource_checksum "${resource_kind}" "${name}")\"")
  done
  local joined
  joined="$(IFS=,; echo "${pairs[*]}")"
  kubectl -n "${NS}" patch "${resource}" --type merge \
    -p "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{${joined}}}}}}" >/dev/null
}

annotate_secret_checksums() {
  local resource="$1"
  shift
  annotate_data_resource_checksums secret "${resource}" "$@"
}

annotate_configmap_checksums() {
  local resource="$1"
  shift
  annotate_data_resource_checksums configmap "${resource}" "$@"
}

apply_manifests() {
  log "creating namespace and provisioning secrets"
  ensure_namespace
  ensure_secret
  ensure_app_db_secret
  ensure_connect_secret
  ensure_database_tls_secret
  apply_manifest_files "${STATIC_MANIFESTS[@]}"
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
  local complete
  complete="$(kubectl -n "${NS}" get job broker-backfill -o jsonpath='{range .status.conditions[?(@.type=="Complete")]}{.status}{end}' 2>/dev/null || true)"
  if [[ "${FORCE_BACKFILL}" != "1" && "${complete}" == "True" ]]; then
    log "broker backfill already complete"
    kubectl -n "${NS}" patch job broker-backfill --type=json \
      -p='[{"op":"remove","path":"/spec/ttlSecondsAfterFinished"}]' >/dev/null 2>&1 || true
    return 0
  fi

  log "running broker backfill"
  local manifest
  manifest="$(mktemp)"
  trap 'rm -f "${manifest}"' RETURN
  select_manifest_documents "${BROKER_MANIFEST}" only Job broker-backfill > "${manifest}"
  kubectl -n "${NS}" delete job broker-backfill --ignore-not-found --wait=true >/dev/null
  kubectl apply -f "${manifest}" -n "${NS}" >/dev/null
  trap - RETURN
  rm -f "${manifest}"
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

roll_out_stack() {
  log "applying infra dependencies"
  apply_manifest_files "${INFRA_MANIFESTS[@]}"
  apply_broker_infra_manifest
  annotate_secret_checksums statefulset/storage storage-secret
  annotate_secret_checksums statefulset/cache cache-secret
  annotate_secret_checksums statefulset/search search-secret
  wait_for_rollouts "${ROLL_OUT_INFRA[@]}"

  log "applying database"
  apply_manifest_files "${DATABASE_MANIFESTS[@]}"
  annotate_secret_checksums statefulset/database database-secret
  wait_for_rollouts "${ROLL_OUT_DATABASE[@]}"

  log "applying application services"
  apply_app_manifests
  kubectl -n "${NS}" set env deployment/frontend ORIGIN="${LOCAL_ORIGIN}" >/dev/null
  annotate_secret_checksums deployment/backend \
    database-secret app-db-secret storage-secret cache-secret backend-secret search-secret
  wait_for_rollouts "${ROLL_OUT_REST[@]}"

  provision_meili_connect_key

  log "checking connect inputs"
  annotate_secret_checksums deployment/connect connect-secret
  annotate_configmap_checksums deployment/connect broker-pipelines
  wait_for_rollouts "${ROLL_OUT_CONNECT[@]}"

  run_broker_backfill
}

start_port_forward_background() {
  local supervisor_pid

  # Stop an existing frontend port-forward on this port before replacing it.
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
  # shellcheck disable=SC2016 # The child shell expands env passed through env.
  env LOCAL_PORT="${LOCAL_PORT}" REMOTE_PORT="${REMOTE_PORT}" NS="${NS}" nohup bash -c '
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
init_image_tags

if [[ -n "$(port_pids)" ]]; then
  echo "note: local port ${LOCAL_PORT} is already in use; deploy will reuse a frontend port-forward or report the conflict." >&2
fi

build_images
apply_manifests
roll_out_stack
start_port_forward_background
print_summary
