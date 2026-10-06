#!/usr/bin/env bash
# A load burst for the brief's Monitor step: extra traffic to both copies of GreenCharge for a
# while, on top of the witness's steady 5 requests a second.
# Usage: deploy/burst.sh [seconds] [parallel clients per copy]
set -euo pipefail
SECS="${1:-60}"; CLIENTS="${2:-6}"
END=$((SECONDS + SECS))
hit() { while [ $SECONDS -lt $END ]; do curl -s -o /dev/null -m 2 -X POST "$1/route" -H 'Content-Type: application/json' -d '{}' || true; curl -s -o /dev/null -m 2 "$1/carbon" || true; done; }
for port in 35555 35556; do for _ in $(seq 1 "$CLIENTS"); do hit "http://localhost:$port" & done; done
wait
echo "burst finished after ${SECS}s"
