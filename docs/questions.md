# Questions the jury may ask

Short answers you can say out loud. The file to point at is in brackets.

**Is the demo real or staged?**
Real. The cluster is the challenge's own. The requests are real HTTP requests, counted by a
separate measuring tool that never touches the app. The grid numbers are published figures for
29 September. The one thing we change is the speed of the clock.
[`docs/demo/run.json`, `data/README.md`]

**Why doesn't ENACT do this already?**
Its policy operator's permissions cover policies and nodes only, not apps. It writes its choice into the
policy's status and stops there, and the plug-in pins the app once at deploy time. That looks
like a deliberate split between deciding and acting, and the acting half had not been written.
[`docs/platform-behaviour.md`]

**Isn't this just the Kubernetes descheduler, or a rolling restart?**
The descheduler evicts pods and lets the scheduler choose again; it knows nothing about ENACT's
policy and it stops the old pod first. Sunflower follows ENACT's named choice, starts the new
copy before stopping the old one, and has rules about when not to move.

**How do you lose zero requests?**
Four things. The new copy starts first. It calls its own endpoints 40 times before it reports
ready, so its first real request does not land on a cold JVM. The old copy is then told to
drain: it stops taking new connections and asks existing clients to reconnect. Last, it shuts
down gracefully. We added the warm-up and the drain after measuring losses without them: 4 of
529 requests in one run on a busy laptop. The three runs since lost none.
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
waits between moves, and it has an hourly limit. In the recorded run, ENACT changed its pick
twice and Sunflower moved once.

**How much carbon does this save?**
We don't claim a number. On this cluster the energy figures are model estimates inside a
virtual machine, nearly identical for every node. What we can measure is time spent out of
policy, so that is what we report.

**Why does Sunflower need 12 seconds?**
Eight of them are a deliberate wait, to be sure the drop is real and not a blip. The move
itself takes about three and a half: start a second copy on the other machine, let it warm
up, switch the traffic, stop the first. The app answers every request throughout. The wait is
a setting. At zero the total would be about four seconds, and it would then move for every
short dip.

**What if ENACT is slow to change its pick?**
Then Sunflower waits, because it only goes where ENACT says. We saw this once: ENACT's
monitor service had been restarted for answering a health check late, and its pick froze for
18 seconds. Sunflower moved as soon as the pick changed. The screen shows that delay
separately, so it is clear whose time it was. We now give that service more room, and it is
finding 40 in our report.

**Why does the screen say 6 hours and 24 seconds when the replay shows 48 s and 12 s?**
The replay runs the day 450 times faster: half an hour of grid data every four seconds. The
standard build's 48 seconds are the whole dirty stretch, so at real speed they are six hours.
Sunflower's 12 seconds do not scale the same way, because its wait and its move take the same
time at any speed. So the fair comparison is not four to one. With the
default 20-second wait and the 3.5-second move we measured, that is about 24 seconds, plus
however long ENACT takes to change its pick. The screen shows both real-speed figures under the
timers. The six hours follow from the data; the 24 seconds are worked out, not yet run.

**Did you do the dataspace step?**
Yes, through the plug-in: connected as the consumer, found the carbon-intensity asset in the
provider's catalogue, negotiated the contract and transferred the file. Both copies of
GreenCharge in the cluster read it, and the app's badge says "live (dataspace file)". The
machines' green share on our screen is a different input: that comes from public British grid
data, replayed.

**What did you use ENACT's assistant for?**
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

**Where do the Application Controller's inputs come from?**
The green share and the region are read from the machine's own labels. The CPU, memory and
network figures are fixed at values that pass the policy, because our measuring tool does not
collect them. So the verdict on screen turns only on green share and region, and we say that
in the code.
