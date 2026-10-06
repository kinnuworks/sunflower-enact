#!/usr/bin/env bash
# Turns the heavier observability UIs on or off: Grafana, Alertmanager, Hubble UI and relay.
# Prometheus, Kepler and the ENACT components are never touched.
set -euo pipefail
CTX=kind-enact-dev
k() { kubectl --context "$CTX" "$@"; }
case "${1:-}" in
  on)  n=1 ;;
  off) n=0 ;;
  *) echo "usage: $0 on|off" >&2; exit 2 ;;
esac
k scale -n enact deploy/infra-grafana --replicas="$n" >/dev/null
k patch alertmanager -n enact infra-kube-prometheus-stac-alertmanager --type=merge -p "{\"spec\":{\"replicas\":$n}}" >/dev/null
k scale -n kube-system deploy/hubble-ui deploy/hubble-relay --replicas="$n" >/dev/null
echo "dashboards $1"
