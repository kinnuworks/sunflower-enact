#!/usr/bin/env bash
# Sets the stage for a run: rewinds the grid replay, then deploys both copies of GreenCharge
# "now", so each starts on the node ENACT chooses at that moment. The standard build is pinned
# there once, as the ENACT SDK does; the other copy is left for Sunflower to place.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(dirname "$HERE")"
CTX=kind-enact-dev; WITNESS=http://localhost:35590
# GreenCharge's carbon feed is the file transferred from the ENACT dataspace. The folder mounted
# into both copies gets a fresh copy of it on every staging.
FEED=grid-carbon-intensity.json
cp "$ROOT/greencharge/dataspace/$FEED" "$HERE/.grid/$FEED"
k() { kubectl --context "$CTX" "$@"; }
h() { helm --kube-context "$CTX" "$@"; }

curl -fsS -X POST "$WITNESS/grid/reset" >/dev/null
# Put every node's green share back to the replay's opening figure. Older builds of the replay
# skip a label they believe is unchanged, which leaves a stress test's value in place.
curl -fsS "$WITNESS/grid/state" | python3 -c '
import json, sys, urllib.request
for n in json.load(sys.stdin)["nodes"]:
    body = json.dumps({"add": {"enact.eu/green-ratio": "%.2f" % n["greenRatio"]}}).encode()
    req = urllib.request.Request("http://localhost:35580/api/v1/nodes/%s/labels" % n["node"], body, {"Content-Type": "application/json"})
    urllib.request.urlopen(req, timeout=10).read()
'
# The policy operator re-ranks on its own schedule; we have seen it take over a minute after a
# label change. With both workers above the policy's minimum it settles on the greener one, so
# wait until its choice is that node, three readings in a row, before pinning anything to it.
greenest() { k get nodes -l enact.eu/green-ratio -o jsonpath='{range .items[*]}{.metadata.labels.enact\.eu/green-ratio}{" "}{.metadata.name}{"\n"}{end}' | sort -rn | head -1 | cut -d' ' -f2; }
CHOSEN=""; SAME=0
for _ in $(seq 1 120); do
  CHOSEN="$(k get runtimepolicy -n enact greencharge-standard -o jsonpath='{.status.chosenNode}')"
  if [ -n "$CHOSEN" ] && [ "$CHOSEN" = "$(greenest)" ]; then SAME=$((SAME + 1)); else SAME=0; fi
  [ "$SAME" -ge 3 ] && break
  sleep 2
done
[ "$SAME" -ge 3 ] || echo "warning: the policy operator has not settled on the greener node; starting from its current choice, $CHOSEN" >&2
[ -n "$CHOSEN" ] || { echo "the policy operator has not chosen a node" >&2; exit 1; }

h upgrade --install standard "$ROOT/greencharge/chart" -n enact \
  --set carbonFeed.hostPath=/grid --set "carbonFeed.file=$FEED" --set "nodeSelector.kubernetes\.io/hostname=$CHOSEN" >/dev/null
h upgrade --install sunflower "$ROOT/greencharge/chart" -n enact \
  --set carbonFeed.hostPath=/grid --set "carbonFeed.file=$FEED" --set service.nodePort=32586 \
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
