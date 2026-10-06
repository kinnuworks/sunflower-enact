package eu.enact.greencharge.adaptation;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;

import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.DTO.ResourceMetrics;
import com.informationcatalyst.enact.application_controller.policymodel.Reconciler.models.PolicyModel;

import eu.enact.greencharge.adaptation.AdaptationDecision.Check;

@SpringBootTest(classes = {AdaptationConfig.class, AdaptationService.class})
class AdaptationServiceTest {

    @Autowired
    AdaptationService adaptation;

    @Autowired
    PolicyModel policy;

    @Test
    void policyModelIsLoadedFromTheReconciliationFile() {
        // The loader fails silently, so a missing or mis-keyed file would otherwise go unnoticed.
        assertThat(policy.getApplication().getPerformance().getCpu().getCores().getMin()).isEqualTo(1);
        assertThat(policy.getApplication().getPerformance().getCpu().getCores().getMax()).isEqualTo(4);
        assertThat(policy.getApplication().getPerformance().getRam().getCapacity()).isEqualTo("2Gi");
        assertThat(policy.getApplication().getNetwork().getCapacity().getLatency()).isEqualTo("50ms");
        assertThat(policy.getApplication().getEnergy().getGreenEnergyMix()).isEqualTo(0.60);
        assertThat(policy.getApplication().getEnergy().getConsumption()).isEqualTo("100W");
        assertThat(policy.getApplication().getDataStorage().getLocations()).containsExactly("eu-west");
    }

    @Test
    void properlyProvisionedNodeIsCompliantWithNoAction() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 2, "2Gi", 40), 0.85, "eu-west", 12.0));

        assertThat(decision.compliant()).isTrue();
        assertThat(decision.action()).isEqualTo("no_action");
        assertThat(decision.recommendation().isCompliant()).isTrue();
        assertThat(decision.recommendation().getCpu().getAction()).isEqualTo("no_action");
    }

    @Test
    void underProvisionedNodeIsNonCompliantAndRecommendsScaleUp() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 0.5, "2Gi", 40), 0.85, "eu-west", 12.0));

        assertThat(decision.compliant()).isFalse();
        assertThat(decision.action()).isEqualTo("scale_up");
        assertThat(decision.recommendation().getCpu().getAction()).isEqualTo("scale_up");
    }

    @Test
    void overProvisionedNodeRecommendsScaleDown() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 8, "2Gi", 40), 0.85, "eu-west", 12.0));

        assertThat(decision.compliant()).isFalse();
        assertThat(decision.action()).isEqualTo("scale_down");
    }

    @Test
    void nodeBelowTheGreenEnergyMixMustBeRelocatedEvenWhenResourcesAreFine() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 2, "2Gi", 40), 0.30, "eu-west", 12.0));

        // The Application Controller on its own still calls this node compliant.
        assertThat(decision.recommendation().isCompliant()).isTrue();
        assertThat(decision.compliant()).isFalse();
        assertThat(decision.action()).isEqualTo("relocate");
        assertThat(decision.reason()).contains("green energy mix").contains("30%").contains("60%");
    }

    @Test
    void relocationTakesPriorityOverScaling() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 0.5, "2Gi", 40), 0.30, "eu-west", 12.0));

        assertThat(decision.action()).isEqualTo("relocate");
    }

    @Test
    void nodeOutsideThePolicyRegionMustBeRelocated() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("remote", 2, "2Gi", 40), 0.90, "us-east", 12.0));

        assertThat(decision.action()).isEqualTo("relocate");
        assertThat(decision.sustainability()).filteredOn(Check::violated).extracting(Check::clause)
                .containsExactly("region");
    }

    @Test
    void powerAboveThePolicyCeilingMustBeRelocated() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 2, "2Gi", 40), 0.85, "eu-west", 140.0));

        assertThat(decision.action()).isEqualTo("relocate");
    }

    @Test
    void unknownSustainabilityValuesAreReportedAsNotAssessedRatherThanGuessed() {
        AdaptationDecision decision = adaptation.evaluate(
                new NodeTelemetry(metrics("enact-dev-worker", 2, "2Gi", 40), null, null, null));

        assertThat(decision.compliant()).isTrue();
        assertThat(decision.action()).isEqualTo("no_action");
        assertThat(decision.sustainability()).extracting(Check::status).containsOnly("not_assessed");
    }

    @Test
    void powerCeilingUnitsAreParsed() {
        assertThat(AdaptationService.watts("100W")).isEqualTo(100.0);
        assertThat(AdaptationService.watts("1KW")).isEqualTo(1000.0);
        assertThat(AdaptationService.watts("lots")).isNull();
    }

    private static ResourceMetrics metrics(String node, double cores, String memory, long latencyMs) {
        ResourceMetrics.CpuMetrics cpu = new ResourceMetrics.CpuMetrics();
        cpu.setCores(cores);
        ResourceMetrics.MemoryMetrics mem = new ResourceMetrics.MemoryMetrics();
        mem.setCapacity(memory);
        ResourceMetrics.NetworkMetrics net = new ResourceMetrics.NetworkMetrics();
        net.setBandwidthMbps(1000);
        net.setLatencyMs(latencyMs);
        ResourceMetrics m = new ResourceMetrics();
        m.setNodeName(node);
        m.setCpu(cpu);
        m.setMemory(mem);
        m.setNetwork(net);
        return m;
    }
}
