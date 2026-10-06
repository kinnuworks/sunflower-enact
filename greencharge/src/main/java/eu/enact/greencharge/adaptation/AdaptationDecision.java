package eu.enact.greencharge.adaptation;

import java.util.List;

import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.DTO.AdaptationRecommendation;

/**
 * The outcome of evaluating one node against the Application Policy Model.
 *
 * @param nodeName       node that was evaluated
 * @param compliant      true only if the Application Controller and every assessed sustainability check pass
 * @param action         the single action to take: {@code no_action}, {@code scale_up}, {@code scale_down} or {@code relocate}
 * @param reason         one plain-language sentence explaining the action
 * @param recommendation the Application Controller's own verdict on CPU, memory and network, unmodified
 * @param sustainability checks on the fields the controller parses but does not evaluate
 */
public record AdaptationDecision(
        String nodeName,
        boolean compliant,
        String action,
        String reason,
        AdaptationRecommendation recommendation,
        List<Check> sustainability) {

    public static final String NO_ACTION = "no_action";
    public static final String SCALE_UP = "scale_up";
    public static final String SCALE_DOWN = "scale_down";
    public static final String RELOCATE = "relocate";

    /**
     * One policy clause compared with one observed value.
     *
     * @param status {@code met}, {@code violated} or {@code not_assessed}
     */
    public record Check(String clause, String required, String observed, String status) {

        public static final String MET = "met";
        public static final String VIOLATED = "violated";
        public static final String NOT_ASSESSED = "not_assessed";

        public boolean violated() {
            return VIOLATED.equals(status);
        }
    }
}
