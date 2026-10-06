# How the ENACT platform behaves on this cluster

Observed on 2026-10-06 on the challenge's 3-node Kind cluster (Apple silicon, OrbStack),
with the images pinned in `deploy/versions.env`. Everything here was run, not inferred.
Sunflower's design follows from this table.

## Getting a placement decision at all

As installed by the challenge `make setup`, a `RuntimePolicy` never gets a `chosenNode`; its
status stays at `reason: no available metrics`. Three separate things cause that:

| # | Cause | Observed | What we did |
|---|---|---|---|
| 1 | `make setup` waits 120 s for every pod, then stops with an error on a first run (image pulls take longer). The node-labelling step after it never runs. | `make: *** [setup] Error 1`; nodes had no `enact.eu/*` labels | Wait until ready, then label |
| 2 | The monitoring agent is installed with an empty join token, because the Makefile reads the token secret before the monitor API has created it. | Agent log: `403 Invalid join token`; `GET /clusters` returned `[]` | Re-install the agent with the real token |
| 3 | The policy operator is installed with `METRICS_API_URL` and `CLUSTER_NAME` empty. | Operator env shows both blank | Set them at install |
| 4 | The monitor API requires a Bearer token on `/latency/*` and `/availability/*`; the policy operator sends none. | `401` without a token, `200` with the admin token | `deploy/metrics-bridge.yaml`, a reverse proxy that adds the token |

`helm upgrade` on the policy operator's chart fails (`map[] does not contain declared merge
key: name`) because the chart renders an image pull secret with an empty name, so item 3 has
to be set at install time or by a JSON patch.

With all four addressed, a policy gets a `chosenNode` within a few seconds.

## How the choice reacts when a node's green ratio changes

Two workers, both in region `eu-west`. The chosen node's `enact.eu/green-ratio` label was
dropped from 0.90 to 0.30 while the other stayed at 0.85.

| Policy | Result |
|---|---|
| Soft green, Hard region (the brief's policy) | `chosenNode` switched to the other worker within 10 s |
| Hard green | `chosenNode` switched within 5 s |
| Soft green only, no Hard constraint | `chosenNode` did not change in 70 s; it stayed on the 0.30 node |

So with the brief's policy the operator does keep its choice up to date. What is missing is
anything that acts on the new choice: the SDK pins a Deployment to `chosenNode` once, at
deploy time.

## Ties

With both workers at 0.90, the two policies that re-rank on every pass changed their choice
back and forth: 2 switches each in 150 s, and they disagreed with each other for part of that
time. A controller that simply followed `chosenNode` would move the app back and forth
between two equally good nodes. Sunflower therefore follows a new choice only when the
current node breaks the policy or the new node is better by a margin, and only after the
choice has held steady.

## No node qualifies

With both workers at 0.30 under Hard green, `chosenNode` is empty, the condition is
`Available=False/Pending`, and the reason reads `green ratio 0.00 below min 0.60` (the
reported ratio is 0.00 although the label is 0.30). Under Soft green a node is still chosen.
Sunflower treats an empty choice as "no decision" and leaves the app where it is.

## Energy and autoscaling inputs

- The monitor API reports about 85 W for each worker. The two values are nearly identical and
  the cluster runs in a VM, so these are model estimates, not measurements. We do not
  headline them.
- `metrics-server` is installed but never becomes ready on Kind (no `--kubelet-insecure-tls`),
  so the standard horizontal autoscaler cannot work on this cluster as shipped.

## What Sunflower does on this platform (measured)

From `deploy/soak.sh`, 5 requests per second to each copy, on 2026-10-06 and 2026-10-07:

| Measure | Result |
|---|---|
| Forced moves | 30 of 30 completed |
| Requests to the moving copy during the run | 2,632, of which 0 failed |
| Time per move (new pod ready, old pod gone) | median 8.8 s, slowest 15.3 s |
| Standard build over the same period | 2,632 requests, 0 failed, never left its first node |

Raw data: `evidence/moves-30.json`. An earlier run on a quieter machine (`evidence/moves-20.json`)
gave a median of 4.8 s per move; move time follows how busy the host is.

One run is deliberately not reported as a result: while Eclipse was being installed on the
same laptop, both copies saw failed requests, including the standard build, which never
moves. Failures on a copy that never moves measure the host, not the mover.

A live scene between those two runs did show one request lost just after a move: a client
reused a keep-alive connection to the old pod as it exited. GreenCharge now tells clients to
close their connection while it drains (`PlacementInfo.drain`), and the 30-move run above was
made after that change.

Two failure cases were also exercised:

- **Target node cannot run the app** (node cordoned). The new pod stayed pending, the move was
  undone after the timeout, and the old copy served throughout: 0 failed requests. The policy
  operator had still chosen the cordoned node; it does not look at whether a node is
  schedulable.
- **Controller killed in the middle of a move.** The replacement controller read its notes
  from the Deployment's annotations and the move completed: 0 failed requests.
