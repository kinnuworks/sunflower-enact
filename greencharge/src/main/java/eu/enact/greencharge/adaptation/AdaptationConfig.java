package eu.enact.greencharge.adaptation;

import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Import;

import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.config.PolicyModelConfig;
import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.services.ComplianceAndAdaptationService;

/**
 * Wires in the two ENACT Application Controller classes GreenCharge uses.
 *
 * <p>They are imported by name rather than component-scanned: the library ships no
 * auto-configuration, and scanning its root package starts schedulers, datasources and
 * message listeners that this service has no use for.
 */
@Configuration
@Import({PolicyModelConfig.class, ComplianceAndAdaptationService.class})
public class AdaptationConfig {
}
