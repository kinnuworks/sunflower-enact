#!/usr/bin/env bash
# Turns the observability extras on or off: Grafana, Alertmanager, Hubble UI and relay,
# kube-state-metrics and metrics-server (which never becomes ready on Kind as installed).
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
k scale -n enact deploy/infra-kube-state-metrics --replicas="$n" >/dev/null
k scale -n kube-system deploy/metrics-server --replicas="$n" >/dev/null 2>&1 || true
echo "dashboards $1"
