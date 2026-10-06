# On a fresh cluster, a RuntimePolicy never gets a `chosenNode`

**What happens.** After `make setup` on a clean machine, every `RuntimePolicy` stays at
`reason: no available metrics` and `status.chosenNode` is empty, so the Deploy step cannot pin
anything.

**Why.** Four independent causes; fixing any three is not enough.

1. `make setup` waits 120 s for all pods and exits with an error when image pulls take longer,
   which they do on a first run. The node-labelling step after the wait never runs.
   *Fix:* wait until ready (or a much longer timeout), then label.
2. The Makefile reads the monitor API's join-token secret before the monitor API has created
   it, so the agent is installed with an empty token. Agent log: `403 Invalid join token`;
   `GET /clusters` returns `[]`.
   *Fix:* wait for the secret to exist before installing the agent.
3. The policy operator is installed with `METRICS_API_URL` and `CLUSTER_NAME` empty. (Its
   README names the variable `MONITOR_API_URL`, which the code does not read.)
   *Fix:* set both in `values/applpm-values.yaml`.
4. The monitor API requires a Bearer token on `/latency/*` and `/availability/*`; the operator
   sends none and gets `401`.
   *Fix:* either let the operator send a token, or exempt those routes for in-cluster callers.
   We used a small reverse proxy that adds the token (`deploy/metrics-bridge.yaml`).

Also: `helm upgrade` on the operator chart fails with `map[] does not contain declared merge
key: name`, because the chart renders an image pull secret entry with an empty name. Values
therefore have to be right at install time.

**A working sequence** is in [`deploy/up.sh`](../deploy/up.sh).
