# Sunflower

Sunflower moves a running app to the computer with the cleanest power, without losing a request.

Built for Veles Hack 2026, Challenge 3 (ENACT, Kubernetes Dynamic Adaptation).

![Two copies of the same app over one replayed day of grid data. The standard build spends 48 seconds on dirty power and never leaves Machine A. The copy with Sunflower spends 12 seconds on dirty power, then moves to Machine B. Each was sent 507 requests and lost none.](docs/img/race.png)

The picture is one real run on the challenge's three-node ENACT cluster. Two copies of
GreenCharge receive the same traffic, five requests a second each. Machine A is fed the green
share of North East England's grid and Machine B the North West's, from the published figures
for 29 September 2026, replayed so that half an hour passes every 4 seconds. A sunflower turns
to face the sun through the day, and this controller does the same for an app and clean power,
which is where the name comes from.

| | Standard build | With Sunflower |
|---|---|---|
| Time on dirty power | 48 s, about 6 hours of the real day | 12 s, about 1.5 hours |
| Requests lost | 0 of 507 | 0 of 507 |
| When Machine A fell below 60% green | stayed on it | moved to Machine B in 3.5 s |
| What the app's own Application Controller check said | `relocate`, for 48 s | `relocate`, for 12 s |

Of Sunflower's 12 seconds, 8 are a deliberate wait to be sure the drop is real, and 3.5 are the
move: start a second copy on Machine B, let it warm up, switch the traffic, stop the first.
Both copies answer every request the whole time. The wait is a setting (`--settle`).

The run is saved as measured in [`docs/demo/run.json`](docs/demo/run.json). To watch it play back:

```bash
python3 -m http.server 8000 --directory docs/demo
```

## Why it exists, and how it works

ENACT's policy operator reads a `RuntimePolicy`, ranks the nodes and writes the best one to
`status.chosenNode`. It keeps that answer current. In our tests, when a node's green share fell
below the policy's minimum, the operator named another node within 10 seconds.

The app does not follow. The ENACT SDK reads `chosenNode` once, at deploy time, and pins the
Deployment to that node. After that the policy can name Machine B for hours while the app runs
on Machine A, below the 60% it asked for. The Deployment keeps no record of which policy
placed it, so nothing else can step in either.

Sunflower is a Kubernetes controller, about 740 lines of Go, that closes that loop. A
Deployment opts in with one annotation:

```yaml
metadata:
  annotations:
    sunflower.enact.eu/policy: greencharge-sunflower
```

It reads the destination from that policy's `chosenNode` and pins the app there with the same
`nodeSelector` the SDK uses. It has no ranking of its own. The judgement it adds is about
timing:

| Sunflower will not | Because | Flag |
|---|---|---|
| move for a dip that ends quickly | grid figures flicker | `--settle` |
| leave a node that still meets the policy for one under 10% greener | with two near-equal nodes we watched the operator switch its pick twice in 150 s | `--margin` |
| move again straight after a move | each move starts a second copy of the app | `--dwell` |
| move more than 6 times an hour | a faulty feed should not keep the app on the road | `--max-moves-per-hour` |
| stop the old copy before the new one answers | that is how requests get lost | built in |
| leave a failed move half done | it restores the old pin and logs why | `--move-timeout` |

Each action is logged in a sentence, as a Kubernetes Event and at Sunflower's `/receipts`
endpoint; the strip at the bottom of the screen is that log. With `--dry-run` it reports what
it would do and changes nothing. The whole decision is one function with 12 unit tests:
[`sunflower/internal/placement/decide.go`](sunflower/internal/placement/decide.go).

## What we measured

All of it ran on the challenge cluster, on one laptop. Raw files are in [`evidence/`](evidence)
and [`docs/demo/`](docs/demo).

| Test | Result |
|---|---|
| The replayed day, three runs in a row | 506, 507 and 523 requests to each copy, none lost. Standard build: 48 s on dirty power each time. Sunflower: 12, 12 and 13 s, with moves of 3.5, 3.6 and 4.1 s |
| The same day before we warmed the app up | on a busy laptop the move took 14.7 s and 4 of 529 requests to the moving copy were lost. The new copy's first requests took over 2 s on a cold JVM |
| 30 forced moves in a row | all completed. 1,777 requests to the moving copy, none lost. Median move 3.6 s, slowest 5.5 s |
| The same test before the warm-up and the two-core limit | 30 moves, 2,632 requests, none lost. Median move 8.8 s, slowest 15.3 s |
| Destination node unable to run the app | move undone after the timeout. The old copy served throughout, none lost |
| Controller killed in the middle of a move | its replacement read the notes on the Deployment and finished the move, none lost |

A request counts as lost if it has not succeeded within 2 seconds. Traffic is sent at a fixed
rate that does not slow down when the app does, so a stall shows up as lost requests.

The lost requests in the fourth row are why GreenCharge now calls its own endpoints 40 times
before it reports itself ready ([`WarmUp.java`](greencharge/src/main/java/eu/enact/greencharge/placement/WarmUp.java))
and runs with a two-core limit. The three clean runs were made after that change. All four
runs are saved in [`evidence/scene-runs/`](evidence/scene-runs).

## The challenge checklist

Every SDK step was done in the ENACT plug-in for Eclipse. There is a screenshot of each screen
in [`evidence/checklist/`](evidence/checklist); the numbers below refer to those files.

| Brief item | Status | Where |
|---|---|---|
| Policy model: CPU, RAM, latency, power, green mix, region | Done | [`policymodel.yml`](greencharge/src/main/resources/reconciliation/policymodel.yml) |
| Application Controller extension | Done | wizard (01, 02), [`adaptation/`](greencharge/src/main/java/eu/enact/greencharge/adaptation), `POST /adaptation/recommendation` |
| `mvn clean test` passes | Done, 14 tests | both cases the brief names are in [`AdaptationServiceTest`](greencharge/src/test/java/eu/enact/greencharge/adaptation/AdaptationServiceTest.java) |
| RuntimePolicy: Soft green ≥ 0.6, Hard region `eu-west`, availability 0.9 | Done | wizard (03 to 06), [`greencharge.yaml`](greencharge/.enact/policies/greencharge.yaml) |
| Packaging: image `greencharge:1.0`, port 8080, ingress `greencharge.local` | Done | wizard (07 to 10), [`sdk-packaging/`](greencharge/sdk-packaging), [`sdk-manifests/`](greencharge/sdk-manifests) |
| Deploy with the policy, pod placed by the operator | Done | SDK deploy (11 to 18): policy applied, node chosen, Deployment pinned, pod running there |
| Monitor under a load burst | Done | [`monitor-grafana-burst.png`](evidence/monitor-grafana-burst.png), `deploy/monitor.sh`, `deploy/burst.sh` |
| One step through the SDK's assistant (bonus) | Done | `generate_runtime_policy` (19 to 22). Its policy is valid but dropped two fields we asked for, so the wizard's policy is the one in use |
| Dataspace feed | Not done yet | The Dataspaces step needs a connector address and key that are not in the challenge material. We have asked the mentors ([`mentor-message.md`](docs/mentor-message.md)). GreenCharge already reads `carbon.feed.file`; here that file is written from public grid data, and the app's badge says so |

The adaptation service also checks three policy fields that the Application Controller loads
and never compares with anything: green energy mix, power ceiling and region. When one fails
it recommends `relocate`.

## What we found in ENACT along the way

We wrote down each problem we hit, with the fix we used: 39 findings in
[`docs/field-report.md`](docs/field-report.md), and five issue drafts for the ENACT team in
[`upstream/`](upstream). Four that a new user meets on day one:

- On a fresh cluster a policy never gets a `chosenNode`. There are four separate causes: the
  setup script stops before it labels the nodes, the monitoring agent is installed with an
  empty token, two of the operator's settings are blank, and the operator sends no token to
  the monitor API. [`deploy/up.sh`](deploy/up.sh) fixes all four.
- The starter app, rebuilt as the brief asks, answers 401 on every route and its jar is
  376 MB. The Application Controller library brings about 100 dependencies with it, a login
  wall among them. Importing the two classes it needs gives a 23 MB jar that starts in a
  second.
- The SDK's assistant loops on a default Ollama install. Its prompt is about 4,300 tokens and
  Ollama's default context cuts it to 2,050, so the model never sees the request. We watched
  it call one tool 23 times and write a policy for `us-west-2`. With a 16k context it needed
  one call.
- The packaging wizard writes `app.kubernetes.io/version: 1.0` unquoted, and the API server
  rejects all three manifests.

How the operator reacts to a change in green share, for each policy mode, is in
[`docs/platform-behaviour.md`](docs/platform-behaviour.md).

## Run it, and what to keep in mind

You need Docker with about 12 GB of memory, `kind`, `kubectl`, Helm 3, Java 21, Maven and Go.

```bash
./deploy/up.sh      # the ENACT cluster, with the four fixes above
./deploy/apps.sh    # both copies of GreenCharge, Sunflower, the grid replay and the screen
```

Open <http://localhost:35590> and press "Play the day". `./deploy/scene.sh` resets the stage
for another run, `./deploy/soak.sh 30 out.json` repeats the forced-move test, and
`./deploy/record.sh` saves the run you just watched. The organisers' original setup guide is
at [`docs/enact-setup.md`](docs/enact-setup.md).

Limits we know about:

- The three machines are containers on one laptop. Their green share is assigned from real
  regional grid data. It is not measured at the machine.
- The grid data is unedited but replayed 450 times faster than it happened, and the waiting
  times are shortened to match: 8 s to be sure, 30 s between moves. The defaults are 20 s and
  60 s, and on a live grid they would be minutes.
- We make no carbon claim. The energy figures this cluster reports are model estimates inside
  a virtual machine, nearly equal for every node, so we report time on dirty power, which we
  can measure.
- GreenCharge holds no state and runs as one replica. An app with a database needs its data
  moved as well.
- Move time follows the host: 3.5 s on a quiet laptop, and 14.7 s in the worst run we recorded.

| Path | Contents |
|---|---|
| [`sunflower/`](sunflower) | the controller |
| [`witness/`](witness) | traffic, the per-request log and the screen |
| [`gridbridge/`](gridbridge) | replays grid data into ENACT's Label API and GreenCharge's carbon feed |
| [`greencharge/`](greencharge) | the app, its adaptation service, its charts and everything the SDK generated |
| [`deploy/`](deploy) | cluster and scene scripts |
| [`evidence/`](evidence), [`docs/`](docs) | raw results, screenshots, field report, pitch |

Grid data: Carbon Intensity API, National Energy System Operator (NESO), CC BY 4.0; see
[`data/README.md`](data/README.md). The screen is set in Overpass (SIL OFL 1.1). Code:
Apache-2.0, see [`LICENSE`](LICENSE).
