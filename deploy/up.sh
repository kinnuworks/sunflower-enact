#!/usr/bin/env bash
# Builds the ENACT cluster the challenge Makefile builds, with the fixes recorded in
# docs/platform-behaviour.md so that the policy operator actually makes placement decisions.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
CTX=kind-enact-dev
k() { kubectl --context "$CTX" "$@"; }
h() { helm --kube-context "$CTX" "$@"; }
say() { printf '\n== %s\n' "$*"; }

wait_for_secret() {
  for _ in $(seq 1 120); do
    k get secret -n enact "$1" >/dev/null 2>&1 && return 0
    sleep 2
  done
  echo "secret $1 never appeared" >&2; return 1
}

say "Creating the Kind cluster"
mkdir -p "$HERE/.grid"
sed "s|__GRID_DIR__|$HERE/.grid|g" "$HERE/kind-config.tmpl.yaml" > "$HERE/.kind-config.yaml"
kind create cluster --config "$HERE/.kind-config.yaml"
kubectl config use-context "$CTX" >/dev/null

say "Installing metrics-server (with the kubelet flag Kind needs)"
k apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml >/dev/null
k patch deployment metrics-server -n kube-system --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]' >/dev/null

say "Adding Helm repositories"
helm repo add cilium https://helm.cilium.io/ --force-update >/dev/null
helm repo add enact-tdcme https://gitlab.eclipse.org/api/v4/projects/8265/packages/helm/stable --force-update >/dev/null
helm repo add enact-applpm https://gitlab.eclipse.org/api/v4/projects/8268/packages/helm/stable --force-update >/dev/null
helm repo add kepler https://sustainable-computing-io.github.io/kepler-helm-chart --force-update >/dev/null
helm repo update >/dev/null

say "Installing Cilium"
h install cilium cilium/cilium --namespace kube-system --version 1.20.1 \
  --set cluster.name="cloud1" --set cluster.id=1 \
  --set hubble.enabled=true --set hubble.tls.enabled=false \
  --set hubble.relay.enabled=true --set hubble.ui.enabled=true \
  --set hubble.metrics.enableOpenMetrics=true \
  --set hubble.metrics.enabled="{dns,drop,tcp,flow,port-distribution,icmp,httpV2:exemplars=true;labelsContext=source_ip\,source_namespace\,source_workload\,destination_ip\,destination_namespace\,destination_workload\,traffic_direction}" \
  --set envoy.enabled=true --set prometheus.enabled=true --set operator.prometheus.enabled=true \
  --set cni.chainingMode="none" >/dev/null

say "Installing monitoring (Prometheus stack, Kepler)"
k create namespace enact >/dev/null
k create secret generic edc-api-key-secret -n enact --from-literal=EDC_API_KEY="" >/dev/null
h install infra -n enact oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack --version 91.9.0 >/dev/null
h install kepler kepler/kepler --namespace enact \
  --set serviceMonitor.enabled=true --set serviceMonitor.labels.release=infra >/dev/null

say "Installing the TDCME monitor API, then its agent with the real join token"
h install tdcme-api enact-tdcme/monitor-api --namespace enact >/dev/null
wait_for_secret join-token-secret
wait_for_secret admin-token-secret
JOIN_TOKEN="$(k get secret join-token-secret -n enact -o jsonpath='{.data.token}' | base64 -d)"
h install tdcme-agent enact-tdcme/monitor-api-agent --namespace enact \
  --set api.host="http://monitor-api-service.enact.svc.cluster.local" \
  --set api.joinToken="$JOIN_TOKEN" \
  --set cluster.name="dev" \
  --set agent.prometheusHost="http://infra-kube-prometheus-stac-prometheus.enact.svc.cluster.local:9090" >/dev/null

say "Installing the metrics bridge and the policy operator, pointed at it"
k apply -f "$HERE/metrics-bridge.yaml" >/dev/null
h install applpm enact-applpm/appl --namespace enact \
  --set controllerManager.container.env.METRICS_API_URL="http://metrics-bridge.enact.svc.cluster.local" \
  --set controllerManager.container.env.CLUSTER_NAME="dev" >/dev/null

say "Waiting for every component (first run pulls 1-2 GB of images)"
k wait --for=condition=ready pod -n enact --all --timeout=1500s >/dev/null
k rollout status -n kube-system deployment/metrics-server --timeout=300s >/dev/null || true

say "Labelling the worker nodes through the policy operator's Label API"
label() {
  for _ in $(seq 1 60); do
    code="$(curl -s -o /dev/null -w '%{http_code}' -m 5 -X POST "http://localhost:35580/api/v1/nodes/$1/labels" \
      -H 'Content-Type: application/json' -d "$2" || true)"
    [ "$code" = "200" ] && return 0
    sleep 2
  done
  echo "could not label $1" >&2; return 1
}
label enact-dev-worker  '{"add":{"enact.eu/green-ratio":"0.85","enact.eu/role":"edge","enact.eu/region":"eu-west","enact.eu/zone":"eu-west-1a"}}'
label enact-dev-worker2 '{"add":{"enact.eu/green-ratio":"0.9","enact.eu/role":"cloud","enact.eu/region":"eu-west","enact.eu/zone":"eu-west-2a"}}'

if [ "${LEAN:-1}" = "1" ]; then
  say "Pausing dashboards that are not needed between screenshots (LEAN=0 keeps them)"
  # On a laptop the full stack leaves little headroom: an image load on top of it was enough
  # to starve the API server and send the control plane into restart loops. `deploy/dashboards.sh on`
  # brings these back when they are wanted.
  "$HERE/dashboards.sh" off
fi

say "Cluster ready"
k get nodes -L enact.eu/green-ratio,enact.eu/role,enact.eu/region
