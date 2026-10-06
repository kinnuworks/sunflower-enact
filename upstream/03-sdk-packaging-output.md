# SDK 1.5.0: packaging output the cluster rejects, and deploy details

1. **Unquoted version label.** With image tag `1.0`, the Kubernetes manifests contain
   `app.kubernetes.io/version: 1.0`. YAML reads that as a number and the API server rejects
   the Deployment, Service and Ingress: `cannot unmarshal number into ... metadata.labels of
   type string`. *Fix:* quote label values.
2. **Probes on a path the app may not have.** Readiness and liveness are generated as
   `GET /health` on the app port, with a 1 s timeout, no startup probe, and no way to change
   the path in the wizard. The Application Controller wizard in the same SDK recommends
   `/actuator/health/...` on the management port. A JVM app that is slow to answer on a busy
   node is restarted in a loop. *Suggestion:* ask for the path, add a startup probe.
3. **Repository and tag.** Entering `greencharge:1.0` as the repository with the tag empty is
   accepted, which would render `greencharge:1.0:latest`. *Suggestion:* split or flag it.
4. **Ingress with no class.** The Ingress has no `ingressClassName` and the challenge cluster
   has no ingress controller, so it is created and does nothing.
5. **"Rejected" in the deploy result.** The dialog lists the second-ranked node as
   `Rejected`, although it met every rule in the policy. "Not selected" would be clearer.
6. **No link from app to policy.** After deploying, the Deployment carries no label or
   annotation naming the `RuntimePolicy` that placed it. Anything that wants to keep the app
   on the policy's choice later has to be told the pairing separately. *Suggestion:* write an
   annotation such as `enact.eu/runtime-policy: <name>` when pinning.

What worked well: the policy wizard validates against the CRD and shows the YAML before
writing, and the deploy flow previews every object and reports where the pod landed.
