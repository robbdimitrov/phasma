#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-phasma}"
timeout="${TIMEOUT:-180s}"

restart() {
  resource_kind="$1"
  name="$2"
  if kubectl -n "$namespace" get "$resource_kind" "$name" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout restart "$resource_kind/$name"
    kubectl -n "$namespace" rollout status "$resource_kind/$name" --timeout="$timeout"
  fi
}

check_deployment() {
  name="$1"
  kubectl -n "$namespace" wait --for=condition=available "deployment/$name" --timeout="$timeout"
}

restart statefulset database
restart statefulset cache
restart statefulset broker
restart statefulset search
restart statefulset storage

check_deployment backend
check_deployment frontend

echo "dependency restart drill completed for namespace $namespace"
