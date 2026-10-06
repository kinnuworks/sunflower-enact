package eu.enact.greencharge.adaptation;

import java.util.Map;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.web.bind.annotation.RestController;

/** Exposes the adaptation decision so a controller outside the app can ask and act. */
@RestController
@RequestMapping("/adaptation")
public class AdaptationController {

    private final AdaptationService adaptation;

    public AdaptationController(AdaptationService adaptation) {
        this.adaptation = adaptation;
    }

    /** Evaluates one node's telemetry against the Application Policy Model. */
    @PostMapping("/recommendation")
    public AdaptationDecision recommend(@RequestBody NodeTelemetry telemetry) {
        return adaptation.evaluate(telemetry);
    }

    @ExceptionHandler(IllegalArgumentException.class)
    @ResponseStatus(HttpStatus.BAD_REQUEST)
    Map<String, String> badRequest(IllegalArgumentException e) {
        return Map.of("error", e.getMessage());
    }
}
