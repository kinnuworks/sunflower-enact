#!/usr/bin/env bash
# Sets the stage for a run: rewinds the grid replay, then deploys both copies of GreenCharge
# "now", so each starts on the node ENACT chooses at that moment. The standard build is pinned
# there once, as the ENACT SDK does; the other copy is left for Sunflower to place.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(dirname "$HERE")"
CTX=kind-enact-dev; WITNESS=http://localhost:35590
k() { kubectl --context "$CTX" "$@"; }
h() { helm --kube-context "$CTX" "$@"; }

curl -fsS -X POST "$WITNESS/grid/reset" >/dev/null
sleep 8   # let the labels land and the policy operator re-rank
CHOSEN=""
for _ in $(seq 1 30); do
  CHOSEN="$(k get runtimepolicy -n enact greencharge-standard -o jsonpath='{.status.chosenNode}')"
  [ -n "$CHOSEN" ] && break; sleep 2
done
[ -n "$CHOSEN" ] || { echo "the policy operator has not chosen a node" >&2; exit 1; }

h upgrade --install standard "$ROOT/greencharge/chart" -n enact \
  --set carbonFeed.hostPath=/grid --set "nodeSelector.kubernetes\.io/hostname=$CHOSEN" >/dev/null
h upgrade --install sunflower "$ROOT/greencharge/chart" -n enact \
  --set carbonFeed.hostPath=/grid --set service.nodePort=32586 \
  --set sunflower.enabled=true --set sunflower.policy=greencharge-sunflower >/dev/null
# Forget where Sunflower's copy was, so this run starts with a fresh first placement.
k patch deploy -n enact sunflower-greencharge --type=json -p '[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]' >/dev/null 2>&1 || true
k annotate deploy -n enact sunflower-greencharge --overwrite \
  sunflower.enact.eu/last-move- sunflower.enact.eu/moves- sunflower.enact.eu/candidate- \
  sunflower.enact.eu/candidate-since- sunflower.enact.eu/candidate-cause- sunflower.enact.eu/status- >/dev/null
k rollout restart -n enact deploy/sunflower >/dev/null   # clears the on-screen log
k rollout status -n enact deploy/sunflower --timeout=120s >/dev/null
k rollout status -n enact deploy/standard-greencharge --timeout=180s >/dev/null
for _ in $(seq 1 60); do
  [ "$(k get deploy -n enact sunflower-greencharge -o jsonpath='{.spec.template.spec.nodeSelector.kubernetes\.io/hostname}')" = "$CHOSEN" ] && break; sleep 1
done
k rollout status -n enact deploy/sunflower-greencharge --timeout=180s >/dev/null
sleep 12   # let the replaced pods finish draining before the counters start
curl -fsS -X POST "$WITNESS/api/reset" >/dev/null
echo "Both copies are on $CHOSEN. Open $WITNESS and press Play."
