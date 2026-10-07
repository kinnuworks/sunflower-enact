#!/usr/bin/env bash
# Settings that keep the ENACT components up on a busy laptop. deploy/up.sh runs this; it is
# safe to run again.
#
# What it changes, and why (see docs/field-report.md, item 40):
# - The monitor API ships with a 100m CPU limit and a 1 s liveness timeout. Under the policy
#   operator's queries it is throttled, fails the probe and is killed. While it is down the
#   operator logs "Failed to fetch cluster metrics" and keeps its previous choice of node.
# - Every ENACT image is pulled again on each restart (imagePullPolicy: Always), so a restart
#   without a fast connection to the registry takes half a minute or never finishes.
set -euo pipefail
k() { kubectl --context kind-enact-dev -n enact "$@"; }

k patch deploy monitor-api --type=json -p '[
  {"op":"replace","path":"/spec/template/spec/containers/0/resources","value":{"requests":{"cpu":"100m","memory":"128Mi"},"limits":{"cpu":"1","memory":"256Mi"}}},
  {"op":"replace","path":"/spec/template/spec/containers/0/livenessProbe/timeoutSeconds","value":5},
  {"op":"replace","path":"/spec/template/spec/containers/0/livenessProbe/failureThreshold","value":6},
  {"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/timeoutSeconds","value":5},
  {"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"IfNotPresent"}
]' >/dev/null
for d in applpm-controller-manager tdcme-agent; do
  k patch deploy "$d" --type=json -p '[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"IfNotPresent"}]' >/dev/null
done
for d in monitor-api applpm-controller-manager tdcme-agent; do k rollout status deploy/"$d" --timeout=240s >/dev/null; done
echo "ENACT components steadied"
