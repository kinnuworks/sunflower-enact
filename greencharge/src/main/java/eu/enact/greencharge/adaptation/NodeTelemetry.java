package eu.enact.greencharge.adaptation;

import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.DTO.ResourceMetrics;

/**
 * What is known about the node being evaluated.
 *
 * <p>{@code metrics} is the Application Controller's own DTO. The remaining fields are the
 * ones the policy model declares but the controller's compliance service does not look at;
 * each is optional, and a missing value is reported as "not assessed" rather than guessed.
 *
 * @param metrics    CPU, memory and network state, as the Application Controller expects it
 * @param greenRatio share of the node's power that is renewable, 0..1
 * @param region     region the node is in
 * @param powerWatts power the workload is drawing
 */
public record NodeTelemetry(ResourceMetrics metrics, Double greenRatio, String region, Double powerWatts) {
}
