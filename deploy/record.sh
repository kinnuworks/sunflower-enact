#!/usr/bin/env bash
# Saves the run the witness has just measured as one file, and refreshes the stand-alone copy of
# the race screen that plays it back (docs/demo). The saved file is the witness's own data,
# unedited: every request, each machine's green share, the policy operator's pick and
# Sunflower's log.
# Usage: deploy/record.sh [name]     ->  docs/demo/<name>.json   (default: run)
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(dirname "$HERE")"
NAME="${1:-run}"; WITNESS=http://localhost:35590; OUT="$ROOT/docs/demo"
mkdir -p "$OUT"
python3 - "$WITNESS" "$OUT/$NAME.json" <<'PY'
import json, sys, urllib.request
witness, out = sys.argv[1], sys.argv[2]
get = lambda path: json.load(urllib.request.urlopen(witness + path, timeout=10))
tl, state, grid = get("/api/timeline"), get("/api/state"), get("/grid/state")
policy = next((p for p in state["cluster"].get("policies") or [] if p.get("greenMin") is not None), {})
doc = {
    "tl": tl,
    "receipts": state["cluster"].get("receipts") or [],
    "greenMin": policy.get("greenMin", 0.6),
    "zones": {n["node"]: n["zone"] for n in grid["nodes"]},
    "drills": {},
    "grid": {k: grid[k] for k in ("source", "licence", "stepMinutes", "stepSeconds", "length", "dataTime")},
    "totals": state["totals"],
    "failures": state["failures"],
}
json.dump(doc, open(out, "w"), separators=(",", ":"))
for t in sorted(state["totals"], key=lambda t: t["target"]):
    print(f'{t["target"]:>10}: {t["requests"]} requests, {t["failed"]} failed, '
          f'{t["outOfPolicySeconds"]:.0f} s below its green rule, served by {t["byNode"]}')
PY
rm -rf "$OUT/fonts"; cp -R "$ROOT/witness/web/fonts" "$OUT/fonts"; cp "$ROOT/witness/web/index.html" "$OUT/screen.html"
cat > "$OUT/index.html" <<HTML
<!doctype html><meta charset="utf-8"><title>Sunflower: recorded run</title>
<meta http-equiv="refresh" content="0; url=screen.html?recording=$NAME.json">
<a href="screen.html?recording=$NAME.json">Play the recorded run</a>
HTML
echo "saved $OUT/$NAME.json"
