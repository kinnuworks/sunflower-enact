package eu.enact.greencharge.placement;

import java.util.Map;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Answers {@code GET /health} on the application port.
 *
 * <p>The ENACT SDK's Application Packaging wizard generates readiness and liveness probes on
 * this path and port, and does not ask for a different one. Without this endpoint a chart or
 * manifest produced by the wizard deploys a pod that never becomes ready. The richer probes
 * on the management port (see the chart in {@code chart/}) are unaffected.
 */
@RestController
public class HealthEndpoint {

    @GetMapping("/health")
    public Map<String, String> health() {
        return Map.of("status", "UP");
    }
}
