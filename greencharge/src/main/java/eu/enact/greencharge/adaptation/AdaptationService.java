package eu.enact.greencharge.adaptation;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

import org.springframework.stereotype.Service;

import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.DTO.AdaptationRecommendation;
import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.DTO.ResourceMetrics;
import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.models.PolicyModel;
import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.services.ComplianceAndAdaptationService;

import eu.enact.greencharge.adaptation.AdaptationDecision.Check;

/**
 * Evaluates a node against the Application Policy Model and says what to do about it.
 *
 * <p>CPU, memory and network are judged by the ENACT Application Controller. The policy
 * model also declares a green energy mix, a power ceiling and a location; the controller
 * loads those but never compares them with anything, so they are checked here. Resource
 * findings are fixed by scaling; a sustainability or location finding cannot be fixed on
 * the same node, so it becomes {@code relocate}.
 */
@Service
public class AdaptationService {

    private final ComplianceAndAdaptationService controller;
    private final PolicyModel policy;

    public AdaptationService(ComplianceAndAdaptationService controller, PolicyModel policy) {
        this.controller = controller;
        this.policy = policy;
    }

    public AdaptationDecision evaluate(NodeTelemetry telemetry) {
        ResourceMetrics metrics = telemetry.metrics();
        if (metrics == null) {
            throw new IllegalArgumentException("metrics is required");
        }
        AdaptationRecommendation recommendation = controller.checkAndAdapt(metrics);
        List<Check> sustainability = sustainabilityChecks(telemetry);

        boolean relocate = sustainability.stream().anyMatch(Check::violated);
        String cpuAction = recommendation.getCpu() == null ? null : recommendation.getCpu().getAction();

        String action;
        String reason;
        if (relocate) {
            Check first = sustainability.stream().filter(Check::violated).findFirst().orElseThrow();
            action = AdaptationDecision.RELOCATE;
            reason = "%s is %s but the policy requires %s; scaling on this node cannot fix that."
                    .formatted(first.clause(), first.observed(), first.required());
        } else if (AdaptationDecision.SCALE_UP.equals(cpuAction) || AdaptationDecision.SCALE_DOWN.equals(cpuAction)) {
            action = cpuAction;
            reason = recommendation.getCpu().getMessage();
        } else if (!recommendation.isCompliant()) {
            // Memory or network is off but the controller has no scaling action for it.
            action = AdaptationDecision.NO_ACTION;
            reason = "Not compliant, but the finding is not one that scaling or moving resolves.";
        } else {
            action = AdaptationDecision.NO_ACTION;
            reason = "Node satisfies the application policy.";
        }
        boolean compliant = recommendation.isCompliant() && !relocate;
        return new AdaptationDecision(metrics.getNodeName(), compliant, action, reason, recommendation, sustainability);
    }

    private List<Check> sustainabilityChecks(NodeTelemetry telemetry) {
        List<Check> checks = new ArrayList<>();
        PolicyModel.Application app = policy.getApplication();
        if (app == null) {
            return checks;
        }
        PolicyModel.Energy energy = app.getEnergy();
        if (energy != null) {
            double minMix = energy.getGreenEnergyMix();
            checks.add(check("green energy mix", "at least " + percent(minMix), telemetry.greenRatio(),
                    telemetry.greenRatio() == null ? null : percent(telemetry.greenRatio()),
                    v -> v + 1e-9 >= minMix));
            Double maxWatts = watts(energy.getConsumption());
            if (maxWatts != null) {
                checks.add(check("power draw", "at most " + energy.getConsumption(), telemetry.powerWatts(),
                        telemetry.powerWatts() == null ? null : "%.1fW".formatted(telemetry.powerWatts()),
                        v -> v <= maxWatts));
            }
        }
        PolicyModel.DataStorage storage = app.getDataStorage();
        if (storage != null && storage.getLocations() != null && !storage.getLocations().isEmpty()) {
            List<String> allowed = storage.getLocations();
            String region = telemetry.region();
            String status = region == null || region.isBlank() ? Check.NOT_ASSESSED
                    : allowed.stream().anyMatch(region::equalsIgnoreCase) ? Check.MET : Check.VIOLATED;
            checks.add(new Check("region", "one of " + allowed, region, status));
        }
        return checks;
    }

    private static Check check(String clause, String required, Double value, String observed,
            java.util.function.DoublePredicate ok) {
        if (value == null) {
            return new Check(clause, required, null, Check.NOT_ASSESSED);
        }
        return new Check(clause, required, observed, ok.test(value) ? Check.MET : Check.VIOLATED);
    }

    private static String percent(double ratio) {
        return Math.round(ratio * 100) + "%";
    }

    /** Parses the policy's power ceiling ("100W", "1KW"); null if it is not in a form we understand. */
    static Double watts(String consumption) {
        if (consumption == null) {
            return null;
        }
        String s = consumption.trim().toUpperCase(Locale.ROOT);
        try {
            if (s.endsWith("KW")) {
                return Double.parseDouble(s.substring(0, s.length() - 2).trim()) * 1000;
            }
            if (s.endsWith("W")) {
                return Double.parseDouble(s.substring(0, s.length() - 1).trim());
            }
        } catch (NumberFormatException e) {
            return null;
        }
        return null;
    }
}
