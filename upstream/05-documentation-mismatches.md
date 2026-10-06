# Challenge repository: places where the docs and the cluster disagree

- `README.md` gives the policy service's host port as 35080 in four places; the cluster config
  and Makefile use 35580.
- The `RuntimePolicy` example in `README.md` does not match the installed CRD: it uses
  `cpu.cores.min/max`, nests `greenEnergy` under `memory`, and gives `minRatio` as a string.
- `HACKATHON.md` names the AI-assistant tool `generate_deployment`; the SDK's tool is
  `package_application`.
- `HACKATHON.md` asks for CPU architecture `x86_64` while the platform images are ARM64.
- The Monitor step refers to "Energy" and "LoadBalancer" dashboards, and the setup guide says
  Kepler energy tracking is pre-configured in Grafana. The cluster ships only the stock
  Kubernetes dashboards. A four-panel dashboard built from metrics that are present is in
  [`deploy/monitor/dashboard.json`](../deploy/monitor/dashboard.json).
- The Dataspaces step needs a connector Management URL, DSP URL and API key that are not in
  the repository or the install guide.
