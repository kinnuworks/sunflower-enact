package eu.enact.greencharge.placement;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.boot.web.servlet.context.ServletWebServerApplicationContext;
import org.springframework.stereotype.Component;

/**
 * Exercises the app's own endpoints before it reports itself ready.
 *
 * <p>A freshly started JVM answers its first requests slowly: classes are still loading and
 * nothing has been compiled yet. During a move that is exactly when the new copy starts taking
 * the traffic, and on a busy node we measured its first requests taking more than two seconds.
 * Spring Boot only switches readiness to "accepting traffic" after every {@link ApplicationRunner}
 * has returned, so the calls made here are finished before Kubernetes sends the first real one.
 */
@Component
public class WarmUp implements ApplicationRunner {

    private static final Logger log = LoggerFactory.getLogger(WarmUp.class);
    private static final int ROUNDS = 40;
    private static final Duration BUDGET = Duration.ofSeconds(12);
    private static final String TELEMETRY = """
            {"metrics":{"nodeName":"warm-up","cpu":{"cores":2},"memory":{"capacity":"2Gi"},
             "network":{"bandwidth_mbps":1000,"latency_ms":40}},"greenRatio":0.7,"region":"eu-west"}""";

    private final ObjectProvider<ServletWebServerApplicationContext> server;

    public WarmUp(ObjectProvider<ServletWebServerApplicationContext> server) {
        this.server = server;
    }

    @Override
    public void run(ApplicationArguments args) {
        ServletWebServerApplicationContext context = server.getIfAvailable();
        if (context == null || context.getWebServer() == null || context.getWebServer().getPort() <= 0) {
            return; // no real web server, as in the tests
        }
        String base = "http://localhost:" + context.getWebServer().getPort();
        HttpClient client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build();
        long deadline = System.nanoTime() + BUDGET.toNanos();
        int done = 0;
        try {
            for (; done < ROUNDS && System.nanoTime() < deadline; done++) {
                send(client, HttpRequest.newBuilder(URI.create(base + "/route")).POST(HttpRequest.BodyPublishers.ofString("{}")));
                send(client, HttpRequest.newBuilder(URI.create(base + "/carbon")).GET());
                send(client, HttpRequest.newBuilder(URI.create(base + "/chargers")).GET());
                send(client, HttpRequest.newBuilder(URI.create(base + "/adaptation/recommendation")).POST(HttpRequest.BodyPublishers.ofString(TELEMETRY)));
            }
        } catch (Exception e) {
            // Warming up is a courtesy to the first callers; the app is still correct without it.
            log.warn("Warm-up stopped after {} rounds: {}", done, e.toString());
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
            return;
        }
        log.info("Warm-up finished: {} rounds of the app's endpoints before accepting traffic.", done);
    }

    private static void send(HttpClient client, HttpRequest.Builder request) throws Exception {
        client.send(request.header("Content-Type", "application/json").timeout(Duration.ofSeconds(3)).build(),
                HttpResponse.BodyHandlers.discarding());
    }
}
