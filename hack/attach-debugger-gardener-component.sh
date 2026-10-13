#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -e

PORT=2345
usage() {
  echo "Usage: $(basename "$0") [-p PORT] <gardener-operator|gardener-apiserver|gardener-controller-manager|gardener-scheduler|gardener-admission-controller|gardenlet>"
  echo ""
  echo "Attaches a dlv debugger to the gardener-operator, gardener-apiserver, gardener-controller-manager,"
  echo "gardener-scheduler, gardener-admission-controller or gardenlet pod in the local dev setup."
  echo ""
  echo "The respective component must have been built on top of the Skaffold debug support image:"
  echo "  - gardener-operator: 'make operator-debug'"
  echo "  - gardener-apiserver, gardener-controller-manager, gardener-scheduler, gardener-admission-controller:"
  echo "    'make operator-debug' and 'make garden-debug'"
  echo "  - gardenlet: 'make seed-debug'"
  echo ""
  echo "Options:"
  echo "  -p PORT   Local port to use for the dlv debug server and kubectl port-forward (default: 2345)"
  echo "  -h        Show this help message"
}

while getopts "hp:" opt; do
  case $opt in
    h) usage; exit 0 ;;
    p) PORT="$OPTARG" ;;
    *) usage; exit 1 ;;
  esac
done
shift $((OPTIND - 1))

COMPONENT="$1"
case "$COMPONENT" in
  gardener-operator | gardener-apiserver | gardener-controller-manager | gardener-scheduler | gardener-admission-controller)
    # These pods follow the 'app=gardener,role=<name without the gardener- prefix>' convention and run a single
    # container named like the component.
    SELECTOR="app=gardener,role=${COMPONENT#gardener-}"
    CONTAINER="$COMPONENT"
    ;;
  gardenlet)
    SELECTOR="app=gardener,role=gardenlet"
    CONTAINER="gardenlet"
    ;;
  *) usage; exit 1 ;;
esac

POD=$(kubectl get pod -n garden -l "$SELECTOR" -o jsonpath='{.items[0].metadata.name}')
if [ -z "$POD" ]; then
  echo "No pod found for $COMPONENT" >&2
  exit 1
fi

POD_COUNT=$(kubectl get pod -n garden -l "$SELECTOR" -o jsonpath='{.items[*].metadata.name}' | wc -w)
if [ "$POD_COUNT" -gt 1 ]; then
  msg="Warning: more than one $COMPONENT pod found."
  msg="$msg This usually should not happen in the debug setup, except during rollouts."
  msg="$msg Expect breakpoints to not trigger properly."

  printf '%s\n' "$msg"
fi

echo "Attaching dlv to pod $POD"

kubectl exec -n garden "$POD" -c "$CONTAINER" -- /duct-tape/go/bin/dlv attach 1 --headless --listen=":$PORT" --continue --accept-multiclient --api-version=2 &
DLV_PID=$!

sleep 2
echo "Forwarding port $PORT"
kubectl port-forward -n garden "$POD" "$PORT:$PORT"

wait $DLV_PID
