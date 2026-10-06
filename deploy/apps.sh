#!/usr/bin/env bash
# Builds the four images, loads them into the cluster one at a time, and deploys the policies,
# both copies of GreenCharge, Sunflower, the grid replay and the witness.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(dirname "$HERE")"
CTX=kind-enact-dev
k() { kubectl --context "$CTX" "$@"; }

for image in greencharge:1.0 sunflower:dev witness:dev gridbridge:dev; do
  dir="${image%%:*}"
  echo "== building and loading $image"
  docker build -q -t "$image" "$ROOT/$dir" >/dev/null
  kind load docker-image "$image" --name enact-dev >/dev/null 2>&1
done

echo '{"Riverside":95,"Uptown":180,"OldTown":300,"Harbor":150}' > "$HERE/.grid/carbon.json"
k apply -f "$HERE/policies/greencharge.yaml" -f "$HERE/sunflower.yaml" -f "$HERE/gridbridge.yaml" -f "$HERE/witness.yaml" >/dev/null
k rollout status -n enact deploy/sunflower --timeout=180s >/dev/null
k rollout status -n enact deploy/gridbridge --timeout=180s >/dev/null
k rollout status -n enact deploy/witness --timeout=180s >/dev/null
for _ in $(seq 1 60); do curl -fsS -m 3 -o /dev/null http://localhost:35590/grid/state && break; sleep 2; done
"$HERE/scene.sh"
