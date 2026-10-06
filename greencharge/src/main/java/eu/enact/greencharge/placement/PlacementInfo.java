package eu.enact.greencharge.placement;

import java.io.IOException;

import java.util.concurrent.atomic.AtomicBoolean;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.availability.AvailabilityChangeEvent;
import org.springframework.boot.availability.ReadinessState;
import org.springframework.context.ApplicationContext;
import org.springframework.stereotype.Component;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.filter.OncePerRequestFilter;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

/**
 * Says where this copy of GreenCharge is running.
 *
 * <p>The node and pod names come from the Kubernetes Downward API (see the chart). Every
 * response carries them as headers, so anything measuring the service can tell which machine
 * answered each request without asking the cluster.
 */
@RestController
public class PlacementInfo {

    public static final String NODE_HEADER = "X-Served-By-Node";
    public static final String POD_HEADER = "X-Served-By-Pod";

    private final String node;
    private final String pod;
    private final ApplicationContext context;
    private final AtomicBoolean draining = new AtomicBoolean();

    public PlacementInfo(@Value("${NODE_NAME:local}") String node, @Value("${POD_NAME:local}") String pod,
            ApplicationContext context) {
        this.node = node;
        this.pod = pod;
        this.context = context;
    }

    /**
     * Called by the pod's preStop hook before it is stopped.
     *
     * <p>A client holding a keep-alive connection keeps sending requests to this pod even after
     * the Service has stopped routing new connections here, and would see the connection drop
     * when the pod exits. So while draining, every response asks the client to close its
     * connection; the client's next request opens a new one, which lands on the new pod. The
     * call returns after a pause, which is what holds the pod open while that happens.
     */
    @GetMapping("/internal/drain")
    public Placement drain(@RequestParam(defaultValue = "6") int seconds) throws InterruptedException {
        draining.set(true);
        AvailabilityChangeEvent.publish(context, ReadinessState.REFUSING_TRAFFIC);
        Thread.sleep(Math.min(Math.max(seconds, 0), 30) * 1000L);
        return new Placement(node, pod);
    }

    public record Placement(String node, String pod) {
    }

    @GetMapping("/whereami")
    public Placement whereAmI() {
        return new Placement(node, pod);
    }

    @Component
    static class HeaderFilter extends OncePerRequestFilter {

        private final PlacementInfo placement;

        HeaderFilter(PlacementInfo placement) {
            this.placement = placement;
        }

        @Override
        protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
                throws ServletException, IOException {
            response.setHeader(NODE_HEADER, placement.node);
            response.setHeader(POD_HEADER, placement.pod);
            if (placement.draining.get()) {
                response.setHeader("Connection", "close");
            }
            chain.doFilter(request, response);
        }
    }
}
