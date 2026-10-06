package eu.enact.greencharge.placement;

import java.io.IOException;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.springframework.web.bind.annotation.GetMapping;
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

    public PlacementInfo(@Value("${NODE_NAME:local}") String node, @Value("${POD_NAME:local}") String pod) {
        this.node = node;
        this.pod = pod;
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
            chain.doFilter(request, response);
        }
    }
}
