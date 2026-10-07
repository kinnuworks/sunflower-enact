# Field report for the ENACT team

What I ran into while taking GreenCharge through the ENACT toolchain, with the fix I used
for each. Offered as feedback, in the order a new user would meet them.

Each item is marked **Ran** (I reproduced it) or **Read** (found in source or docs, not yet
reproduced).

## Challenge repository (`ENACT-VELESHACK-2026`)

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 1 | The starter app, built as shipped, answers `401` on `/chargers`, `/carbon` and `/route`. The Application Controller dependency brings in Spring Security, whose default is a login wall. The chart's probes hit `/chargers`, so a rebuilt image would never become ready. | Ran | Exclude the controller's transitive dependencies; see item 7 |
| 2 | The built jar is 376 MB and takes about 5.6 s to start. With the exclusions it is 23 MB and starts in about 1 s. | Ran | Same |
| 3 | `README.md` gives the policy service's host port as 35080 in four places. The cluster config and Makefile use 35580. | Read | Use 35580 |
| 4 | The `RuntimePolicy` example in `README.md` does not match the installed CRD: it uses `cpu.cores.min/max`, nests `greenEnergy` under `memory`, and gives `minRatio` as a string. | Read | Generate the policy with the SDK wizard |
| 5 | `HACKATHON.md` names the AI-assistant tool `generate_deployment`. The SDK's tool is `package_application`. | Read | Use `package_application` |
| 6 | `HACKATHON.md` asks for CPU architecture `x86_64` in the policy model, while the platform images are built for ARM64. | Read | Declared as written; not enforced |

## Application Controller 1.0.0

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 7 | The library ships no auto-configuration and about 100 transitive dependencies. A host app needs only `PolicyModelConfig` and `ComplianceAndAdaptationService` for the policy-model reconciler. | Ran | `@Import` those two classes; wildcard-exclude the rest; add `javax.annotation-api` so `@PostConstruct` still runs |
| 8 | The README says to component-scan `eu.enact-horizon`. That is not a legal Java package name; the real root is `com.informationcatalyst.enact.application_controller`. | Read | Import by class |
| 9 | The policy loader reads snake_case keys (`data_storage`, `green_energy_mix`). The sample `policymodel.yml` bundled in the jar is camelCase, so copying it silently drops fields. | Read | Write the policy in snake_case and assert the loaded values in a test |
| 10 | `checkAndAdapt` returns `error` for network, and so never reports compliant, unless the policy has both `network.capacity.bandwidth` and `latency`. | Read | Include both |
| 11 | The policy model's green energy mix, power ceiling and location are loaded but never compared with anything. | Ran | Evaluated in `AdaptationService` on top of the controller's verdict |
| 12 | A missing or malformed policy file is logged and otherwise ignored; the app starts with an empty policy. | Read | Test asserts the policy loaded |

## Cluster setup and platform wiring

Details and evidence are in [platform-behaviour.md](platform-behaviour.md).

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 13 | `make setup` stops with an error on a first run: it waits 120 s for all pods, which is shorter than the image pulls. The node-labelling step never runs. | Ran | Wait until ready, then label |
| 14 | The monitoring agent is installed with an empty join token (the Makefile reads the secret before it exists), so the cluster never registers. | Ran | Re-install the agent with the token |
| 15 | The policy operator is installed with `METRICS_API_URL` and `CLUSTER_NAME` empty, so every `RuntimePolicy` reports `no available metrics`. Its README names the variable `MONITOR_API_URL`, which the code does not read. | Ran | Set both at install |
| 16 | The monitor API requires a Bearer token on the routes the policy operator calls; the operator sends none. | Ran | A reverse proxy that adds the token |
| 17 | `helm upgrade` on the policy operator chart fails because it renders an image pull secret with an empty name. | Ran | Set values at install time |
| 18 | With two equally good nodes, policies with a Hard constraint switch `chosenNode` back and forth every few passes. | Ran | Sunflower only follows a choice that is better and stable |
| 19 | When no node qualifies under Hard green, the reason reports `green ratio 0.00` whatever the node's real ratio is. | Ran | None needed |
| 20 | `metrics-server` never becomes ready on Kind as installed. | Ran | Not used |

## ENACT SDK (Eclipse plug-in, v1.5.0)

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 21 | The Application Packaging wizard generates readiness and liveness probes on `GET /health` on the app port and does not ask for a path. GreenCharge has no such endpoint, so a pod deployed from the generated chart never becomes ready. The Application Controller wizard, in the same SDK, recommends different probes (`/actuator/health/...` on the management port). | Ran | Added a `/health` endpoint to GreenCharge |
| 22 | On the packaging wizard's first page the two text boxes are drawn to the left of their labels ("Application name", "Namespace"), unlike every other page. | Ran | None needed |
| 23 | The packaging wizard accepts `greencharge:1.0` in the Repository box with Tag left empty, which would render the image as `greencharge:1.0:latest`. It could split the value or flag it. | Ran | Entered repository and tag separately |
| 24 | The Application Controller wizard appends `server.port` and `management.server.port` to `application.properties` even when both are already set there. | Ran | Left as written; values match |
| 25 | The policy wizard validates the result against the CRD before writing and shows the YAML first. This worked well and is the step I would point new users to. | Ran | n/a |
| 26 | With image tag `1.0`, the packaging wizard writes the label `app.kubernetes.io/version: 1.0` unquoted in the Kubernetes manifests. YAML reads that as a number and the API server rejects the file: `cannot unmarshal number into ... metadata.labels of type string`. | Ran | Quoted the value in the three generated files; nothing else in them was changed |

### Application Deployment

Deployed from the SDK on 2026-10-06 to the Kind cluster: policy applied, `chosenNode` read,
Deployment pinned with `nodeSelector["kubernetes.io/hostname"]`, pod running on that node.
Screenshots are `evidence/checklist/11` to `18`.

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 27 | The deploy flow worked first time once the cluster was registered: connection test, a preview of every object, then a result naming the chosen node and where the pod is running. The preview marks which object gets pinned. | Ran | n/a |
| 28 | The result dialog says `Rejected: enact-dev-worker`. That node met every rule in the policy (region `eu-west`, green ratio 0.85 against a minimum of 0.60); it only ranked second. "Rejected" reads as "failed the policy". | Ran | None needed; "not selected" would be clearer |
| 29 | The policy status gives the same score to the chosen node and the one it ranked below: `selected based on vector distance score (0.0000)` and `lost in ranking (score 0.0000)`. The reason does not say what separated them. | Ran | None needed |
| 30 | The pin is written once. After the deploy the Deployment carries no reference to its policy (no label or annotation), so nothing can tell later which policy placed it. | Ran | Sunflower's opt-in annotation names the policy |
| 31 | The packaging wizard generates an Ingress with no `ingressClassName`, and the challenge cluster has no ingress controller, so the Ingress is created and does nothing. | Ran | Reached the app through its Service |
| 32 | The file picker for the kubeconfig cannot show `~/.kube`, the default location, because the folder is hidden. | Ran | Typed the path |
| 45 | The deploy wizard offers "Helm chart" as a source and accepts a chart folder through all three pages, then answers "Helm chart deployment is not available yet. Generate Kubernetes manifests from the Packaging module and deploy those instead." The brief asks teams to generate a Helm chart and deploy it. | Ran | Deployed the manifests through the SDK; checked the generated chart with `helm lint` and a server-side dry run |

### AI Assistant

One step done through the assistant on 2026-10-06: `generate_runtime_policy`, with the model
`qwen2.5:3b` from the SDK's own catalogue, running in Ollama 0.40. Screenshots are
`evidence/checklist/19` to `22`; the failed first attempt is kept in `evidence/ai-assistant/`.

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 33 | On Ollama's default settings the assistant loops. The SDK sends about 4,300 tokens of instructions and tool definitions; Ollama's default context is 4,096 and its log shows `truncating input prompt limit=2050 prompt=4308` on every call. The model never sees the user's request. It called `generate_runtime_policy` 23 times and wrote a policy for region `us-west-2`, namespace `default`, green ratio 0.9, with an `aiCompliance` block nobody asked for. A larger model would hit the same limit. | Ran | Created a copy of the same model with `num_ctx` 16384. The step then took one tool call |
| 34 | Each of those calls overwrote the same policy file without asking, and three were marked as failed with no reason shown. The loop ran for 23 calls before it ended; in my screenshot of it running, the Stop button is greyed out. | Ran | Cleared the chat |
| 35 | With the larger context, the generated policy passed the CRD check and the API server accepted it, but it left out two things the request stated: `location.mode: Hard` and `memory.min`. The assistant reported the policy as generated and validated without mentioning what it had dropped. | Ran | Kept the wizard's policy as the one in use |
| 36 | The Install button for Ollama opens the download web page. That page no longer has the download button the SDK's guide describes; the Mac app is behind a small "Download manually" link. | Ran | Used that link |
| 37 | Before Ollama is installed, the model list reads `Could not list models: Request failed after 3 attempt(s): null`. | Ran | None needed |

### Monitoring

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 38 | The brief's Monitor step says to watch the "Energy" and "LoadBalancer" dashboards. The cluster ships Grafana with the stock Kubernetes dashboards only; there is no Energy, LoadBalancer or Kepler dashboard, although the setup guide says Kepler energy tracking is pre-configured. | Ran | Wrote a four-panel dashboard from metrics that are present (`deploy/monitor/dashboard.json`) |
| 39 | The API server, scheduler and controller manager restart in a loop when the laptop is busy, and the cluster does not recover by itself. The SDK-generated probes (1 s timeout, no startup probe) make it worse: each JVM they restart adds load. I lost the cluster twice this way. | Ran | Paused Grafana, Alertmanager, Hubble UI, kube-state-metrics and metrics-server by default (`deploy/dashboards.sh`); gave my own chart a 4 s probe timeout |

| 40 | The monitor API ships with a 100m CPU limit and a liveness probe that times out after 1 s. With the policy operator querying it on every node change it is throttled, fails the probe and is killed (exit 137); I counted six restarts in eight hours. Each restart pulls the image again (`imagePullPolicy: Always`). While it is down the operator logs `Failed to fetch cluster metrics` on every pass and keeps its previous `chosenNode`. In one run that left a policy naming a node below its green minimum for 18 s. | Ran | `deploy/steady.sh`: one CPU and a 5 s probe timeout for the monitor API, `IfNotPresent` for the three ENACT deployments |

### Dataspaces

Done on 2026-10-07 through the SDK: connector added, catalogue browsed, contract negotiated,
file transferred and saved. Screenshots are `evidence/checklist/23` to `32`; the values I
used are in `greencharge/dataspace/README.md`.

| # | What happens | Status | Fix I used |
|---|---|---|---|
| 41 | The brief gives the consumer's Management URL and the provider's DSP URL. The Add connector dialog also needs the consumer's own DSP URL, an API key header and key, and a relay URL, none of which are in the brief, the repository or the install guide. Several teams were stuck at this dialog for a day. | Ran | Key and header from the mentors; DSP URL inferred as the same host with `/api/dsp` |
| 42 | The connector only supports push delivery, so "Download to this computer" is disabled until a Relay URL is set, and nothing says what that URL is. I found it by reading the destination column of earlier completed transfers in the Transfers list: `https://sovity-download.sedimark.work`. | Ran | Entered that address as the Relay URL |
| 43 | The consumer connector is shared, so every team sees every other team's negotiations, agreements and transfer destinations, and the agreement chooser for a new transfer lists all of them. | Ran | Chose my own agreement by its signing time |
| 44 | The flow itself worked first time once the values were in: Detect recognised the API version, the catalogue showed the asset with its terms, the negotiation list refreshed until the agreement was final, and the file arrived in under ten minutes from the first click. | Ran | n/a |
