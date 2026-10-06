# Questions the jury may ask

Short answers you can say out loud. The file to point at is in brackets.

**Is the demo real or staged?**
Real. The cluster is the challenge's own. The requests are real HTTP requests; the screen draws
one stitch per request from the measuring tool's log. The grid numbers are real published data
for 29 September, replayed faster. The only thing we speed up is the clock.
[`docs/demo/run.json`, `data/README.md`]

**Why doesn't ENACT do this already?**
Its policy operator is only allowed to read policies and nodes. It writes its choice into the
policy's status and stops there, and the plug-in pins the app once at deploy time. That looks
like a deliberate split: one part decides, another acts. We built the part that acts.
[`docs/platform-behaviour.md`]

**Isn't this just the Kubernetes descheduler, or a rolling restart?**
The descheduler evicts pods and lets the scheduler choose again; it knows nothing about ENACT's
policy and it stops the old pod first. Sunflower follows ENACT's named choice, starts the new
copy before stopping the old one, and has rules about when not to move.

**How do you lose zero requests?**
Three things. The new copy starts first and must answer health checks before the old one is
touched. The old copy is told to drain: it stops taking new connections and asks existing
clients to reconnect. Then it shuts down gracefully. We found the second step was needed the
hard way: before it, one request in a thousand was dropped.
[`greencharge/.../placement/PlacementInfo.java`]

**What if the new machine is broken?**
The new copy never becomes ready, the timeout passes, and Sunflower puts the pin back. The old
copy served the whole time. We tested it: zero lost requests.

**What if Sunflower itself crashes mid-move?**
Its notes are stored on the app's Deployment, not in memory. The replacement reads them and
finishes the move. Tested: zero lost.

**Won't it bounce back and forth?**
That is what the guard rails are for. It waits for a change to last, it needs the new machine
to be clearly better (10% by default) unless the current one actually breaks the policy, it
waits between moves, and it has an hourly limit. In the run you saw, ENACT changed its pick
three times and Sunflower moved once.

**How much carbon does this save?**
We don't claim a number. On this cluster the energy figures are model estimates inside a
virtual machine, nearly identical for every node. What we can measure is time spent out of
policy, so that is what we report.

**Why 48 seconds and 13? Those are tiny.**
They are replayed seconds. Half an hour of grid data passes every four seconds, so 48 seconds
on screen is about six hours of the real day, against about an hour and a half.

**Did you do the dataspace step?**
Not yet. The plug-in asks for a connector address and key that are not in the challenge
material. We asked the mentors and are waiting. The feed file path is wired, and the app's
badge says honestly where its data comes from: a replay of public grid data.

**What did you use the AI assistant for?**
Generating the runtime policy. On default settings it looped 23 times and wrote a policy for
the wrong region. We traced it to the model's context being too small for the plug-in's
prompt, fixed that, and it worked in one call. It still dropped two fields, so we kept the
wizard's policy. [`docs/field-report.md`, items 33 to 35]

**Would this work for an app that stores data?**
Not as it stands. GreenCharge keeps no state, so a second copy can start anywhere. An app with
a database needs its data to move too. We say so in the README.

**What would you do next?**
Offer the findings and the four setup fixes to the ENACT team, and suggest the plug-in record
which policy placed each app, so tools like this do not need an extra annotation.

**How much of this did AI write?**
We used an AI coding assistant throughout, as most teams will have. What it could not do for us
is run the platform and see what actually happens. The gap we found, the lost-request bug and
the 39 findings all came from running things and reading the results.
