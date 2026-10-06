# Field report for the ENACT team

What we ran into while taking GreenCharge through the ENACT toolchain, with the fix we used
for each. Offered as feedback, in the order a new user would meet them.

Each item is marked **Ran** (we reproduced it) or **Read** (found in source or docs, not yet
reproduced).

## Challenge repository (`ENACT-VELESHACK-2026`)

| # | What happens | Status | Fix we used |
|---|---|---|---|
| 1 | The starter app, built as shipped, answers `401` on `/chargers`, `/carbon` and `/route`. The Application Controller dependency brings in Spring Security, whose default is a login wall. The chart's probes hit `/chargers`, so a rebuilt image would never become ready. | Ran | Exclude the controller's transitive dependencies; see item 7 |
| 2 | The built jar is 376 MB and takes about 5.6 s to start. With the exclusions it is 23 MB and starts in about 1 s. | Ran | Same |
| 3 | `README.md` gives the policy service's host port as 35080 in four places. The cluster config and Makefile use 35580. | Read | Use 35580 |
| 4 | The `RuntimePolicy` example in `README.md` does not match the installed CRD: it uses `cpu.cores.min/max`, nests `greenEnergy` under `memory`, and gives `minRatio` as a string. | Read | Generate the policy with the SDK wizard |
| 5 | `HACKATHON.md` names the AI-assistant tool `generate_deployment`. The SDK's tool is `package_application`. | Read | Use `package_application` |
| 6 | `HACKATHON.md` asks for CPU architecture `x86_64` in the policy model, while the platform images are built for ARM64. | Read | Declared as written; not enforced |

## Application Controller 1.0.0

| # | What happens | Status | Fix we used |
|---|---|---|---|
| 7 | The library ships no auto-configuration and about 100 transitive dependencies. A host app needs only `PolicyModelConfig` and `ComplianceAndAdaptationService` for the policy-model reconciler. | Ran | `@Import` those two classes; wildcard-exclude the rest; add `javax.annotation-api` so `@PostConstruct` still runs |
| 8 | The README says to component-scan `eu.enact-horizon`. That is not a legal Java package name; the real root is `com.informationcatalyst.enact.application_controller`. | Read | Import by class |
| 9 | The policy loader reads snake_case keys (`data_storage`, `green_energy_mix`). The sample `policymodel.yml` bundled in the jar is camelCase, so copying it silently drops fields. | Read | Write the policy in snake_case and assert the loaded values in a test |
| 10 | `checkAndAdapt` returns `error` for network, and so never reports compliant, unless the policy has both `network.capacity.bandwidth` and `latency`. | Read | Include both |
| 11 | The policy model's green energy mix, power ceiling and location are loaded but never compared with anything. | Ran | Evaluated in `AdaptationService` on top of the controller's verdict |
| 12 | A missing or malformed policy file is logged and otherwise ignored; the app starts with an empty policy. | Read | Test asserts the policy loaded |

## Cluster setup and platform wiring

Details and evidence are in [platform-behaviour.md](platform-behaviour.md).

| # | What happens | Status | Fix we used |
|---|---|---|---|
| 13 | `make setup` stops with an error on a first run: it waits 120 s for all pods, which is shorter than the image pulls. The node-labelling step never runs. | Ran | Wait until ready, then label |
| 14 | The monitoring agent is installed with an empty join token (the Makefile reads the secret before it exists), so the cluster never registers. | Ran | Re-install the agent with the token |
| 15 | The policy operator is installed with `METRICS_API_URL` and `CLUSTER_NAME` empty, so every `RuntimePolicy` reports `no available metrics`. Its README names the variable `MONITOR_API_URL`, which the code does not read. | Ran | Set both at install |
| 16 | The monitor API requires a Bearer token on the routes the policy operator calls; the operator sends none. | Ran | A reverse proxy that adds the token |
| 17 | `helm upgrade` on the policy operator chart fails because it renders an image pull secret with an empty name. | Ran | Set values at install time |
| 18 | With two equally good nodes, policies with a Hard constraint switch `chosenNode` back and forth every few passes. | Ran | Sunflower only follows a choice that is better and stable |
| 19 | When no node qualifies under Hard green, the reason reports `green ratio 0.00` whatever the node's real ratio is. | Ran | None needed |
| 20 | `metrics-server` never becomes ready on Kind as installed. | Ran | Not used |

## To be completed

The SDK section is added once its steps have been run in Eclipse.
