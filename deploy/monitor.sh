#!/usr/bin/env bash
# The brief's Monitor step: a Grafana dashboard for the two copies of GreenCharge.
#   deploy/monitor.sh on    load the dashboard, start Grafana, serve it on http://localhost:3000
#   deploy/monitor.sh off   stop Grafana again (it is paused by default to spare the laptop)
# Then run deploy/burst.sh in another terminal and watch the panels react.
# Viewing is opened to anonymous read-only users, so no password is needed on this local cluster.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
k() { kubectl --context kind-enact-dev -n enact "$@"; }
case "${1:-}" in
  on)
    k apply -f "$HERE/monitor/dashboard.json" >/dev/null
    k set env deploy/infra-grafana GF_AUTH_ANONYMOUS_ENABLED=true GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer >/dev/null
    k scale deploy/infra-grafana --replicas=1 >/dev/null
    k rollout status deploy/infra-grafana --timeout=240s >/dev/null
    echo "open http://localhost:3000/d/greencharge-load  (Ctrl-C stops serving it)"
    k port-forward svc/infra-grafana 3000:80 >/dev/null ;;
  off) k scale deploy/infra-grafana --replicas=0 >/dev/null; echo "Grafana stopped" ;;
  *) echo "usage: $0 on|off" >&2; exit 2 ;;
esac
