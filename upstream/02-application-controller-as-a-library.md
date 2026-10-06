# Using `application-controller:1.0.0` as a library breaks the host app

**What happens.** The starter GreenCharge, rebuilt with the dependency as the brief asks,
answers `401` on `/chargers`, `/carbon` and `/route`. Its jar is 376 MB and takes about 5.6 s
to start. The chart's probes hit `/chargers`, so a rebuilt image never becomes ready.

**Why.** The artifact is a full application, not a library: it brings about 100 transitive
dependencies, including Spring Security (whose default is a login wall), and ships no
auto-configuration.

**What worked.** Exclude the transitive dependencies and import the two classes the
policy-model reconciler needs:

```xml
<exclusions><exclusion><groupId>*</groupId><artifactId>*</artifactId></exclusion></exclusions>
```
```java
@Import({PolicyModelConfig.class, ComplianceAndAdaptationService.class})
```

plus `javax.annotation:javax.annotation-api` so `@PostConstruct` still runs. Result: 23 MB,
about 1 s to start.

**Smaller things in the same library**

- The README says to component-scan `eu.enact-horizon`, which is not a legal Java package; the
  real root is `com.informationcatalyst.enact.application_controller`.
- The policy loader reads snake_case keys, but the sample `policymodel.yml` inside the jar is
  camelCase, so copying it silently drops fields.
- `checkAndAdapt` never reports a node compliant unless the policy has both
  `network.capacity.bandwidth` and `latency`.
- Green energy mix, power ceiling and location are loaded from the policy but never compared
  with anything.
- A missing or malformed policy file is logged and otherwise ignored.

**Suggestion.** Publish a small `application-controller-core` artifact with the policy model
and compliance service only, and an auto-configuration for it.
