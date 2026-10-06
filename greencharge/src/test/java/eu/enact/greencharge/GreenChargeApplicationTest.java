package eu.enact.greencharge;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;

/**
 * Starts the whole application, as the container does. The adaptation tests load two classes
 * only, so a bean that clashes with one of Spring's own would pass them and fail in the pod.
 */
@SpringBootTest
@AutoConfigureMockMvc
class GreenChargeApplicationTest {

    @Autowired
    MockMvc mvc;

    @Test
    void servesTheChallengeEndpointsWithoutALoginWall() throws Exception {
        mvc.perform(get("/chargers")).andExpect(status().isOk());
        mvc.perform(get("/carbon")).andExpect(status().isOk()).andExpect(jsonPath("$.source").exists());
        mvc.perform(post("/route").contentType(MediaType.APPLICATION_JSON).content("{}"))
                .andExpect(status().isOk()).andExpect(jsonPath("$.recommended.name").exists());
    }

    @Test
    void everyResponseSaysWhichNodeAnsweredIt() throws Exception {
        mvc.perform(get("/chargers")).andExpect(header().exists("X-Served-By-Node"));
        mvc.perform(get("/whereami")).andExpect(status().isOk()).andExpect(jsonPath("$.node").exists());
    }

    @Test
    void answersTheProbePathTheSdkPackagingWizardGenerates() throws Exception {
        mvc.perform(get("/health")).andExpect(status().isOk()).andExpect(jsonPath("$.status").value("UP"));
    }

    @Test
    void exposesTheAdaptationDecisionOverHttp() throws Exception {
        String body = """
                {"metrics":{"nodeName":"w1","cpu":{"cores":2},"memory":{"capacity":"2Gi"},
                 "network":{"bandwidth_mbps":1000,"latency_ms":40}},"greenRatio":0.3,"region":"eu-west"}""";
        mvc.perform(post("/adaptation/recommendation").contentType(MediaType.APPLICATION_JSON).content(body))
                .andExpect(status().isOk()).andExpect(jsonPath("$.action").value("relocate"));
    }
}
