#!/usr/bin/env bash
# Forces N moves by flipping which worker is green, and records what the witness measured.
# Usage: deploy/soak.sh [moves] [output.json]
set -euo pipefail
N="${1:-20}"; OUT="${2:-evidence/moves.json}"
CTX=kind-enact-dev; API=http://localhost:35580; WITNESS=http://localhost:35590
k() { kubectl --context "$CTX" "$@"; }
label() { curl -fsS -m 10 -o /dev/null -X POST "$API/api/v1/nodes/$1/labels" -H 'Content-Type: application/json' -d "{\"add\":{\"enact.eu/green-ratio\":\"$2\"}}"; }
pinned() { k get deploy -n enact sunflower-greencharge -o jsonpath='{.spec.template.spec.nodeSelector.kubernetes\.io/hostname}'; }
status() { k get deploy -n enact sunflower-greencharge -o jsonpath='{.metadata.annotations.sunflower\.enact\.eu/status}'; }

# Short guard rails so the soak takes minutes, not hours. The defaults are restored at the end.
k patch deploy -n enact sunflower --type=json -p '[{"op":"replace","path":"/spec/template/spec/containers/0/args","value":["--settle=3s","--dwell=4s","--margin=0.10","--max-moves-per-hour=0"]}]' >/dev/null
k rollout status -n enact deploy/sunflower --timeout=90s >/dev/null
label enact-dev-worker 0.85; label enact-dev-worker2 0.90; sleep 12
curl -fsS -X POST "$WITNESS/api/reset"; START=$(date -u +%FT%TZ)

for i in $(seq 1 "$N"); do
  from="$(pinned)"; other=enact-dev-worker; [ "$from" = enact-dev-worker ] && other=enact-dev-worker2
  label "$other" 0.90; label "$from" 0.30
  for _ in $(seq 1 120); do
    [ "$(pinned)" = "$other" ] && [ "$(status)" = "Stay" ] && break
    sleep 0.5
  done
  [ "$(pinned)" = "$other" ] || { echo "move $i did not happen" >&2; exit 1; }
  echo "move $i: $from -> $other"
  sleep 4
done

k patch deploy -n enact sunflower --type=json -p '[{"op":"replace","path":"/spec/template/spec/containers/0/args","value":["--settle=20s","--dwell=60s","--margin=0.10"]}]' >/dev/null
label enact-dev-worker 0.85; label enact-dev-worker2 0.90
mkdir -p "$(dirname "$OUT")"
python3 - "$OUT" "$START" "$N" <<'PY'
import json, sys, urllib.request
out, start, n = sys.argv[1], sys.argv[2], int(sys.argv[3])
state = json.load(urllib.request.urlopen("http://localhost:35590/api/state"))
moved = [r for r in json.loads(json.dumps(state["cluster"].get("receipts") or [])) if r["kind"] == "Moved" and r["time"] >= start]
durs = sorted(round(r["seconds"], 2) for r in moved)
pick = lambda q: durs[min(len(durs)-1, int(len(durs)*q))] if durs else None
import os
result = {"started": start, "movesRequested": n, "movesCompleted": len(durs),
          "hostLoadAverage": os.getloadavg()[0],
          "failures": state.get("failures", []),
          "moveSeconds": {"min": durs[0] if durs else None, "median": pick(0.5), "p95": pick(0.95), "max": durs[-1] if durs else None},
          "traffic": sorted(state["totals"], key=lambda t: t["target"])}
json.dump(result, open(out, "w"), indent=2)
print(json.dumps(result, indent=2))
PY
