# The pitch

Five minutes, strict. About 600 words spoken, so the screen has time to do its work. The
times are where you should be when each part ends.

Before you start: restart the laptop, run `./deploy/scene.sh`, open <http://localhost:35590>
full screen, and keep the slides in a second tab. Leave Eclipse, Ollama and spare browser tabs
closed.

## 0:00 to 0:40, the problem (title slide)

> Hi, I'm ___, and this is Sunflower.
>
> The challenge app is called GreenCharge. It sends electric-car drivers to the charger with
> the cleanest power. ENACT's job is to run GreenCharge on a computer that also has clean
> power: you write a rule, "at least sixty percent green", and ENACT picks the machine.
>
> We did the whole checklist in the ENACT plug-in. Then we noticed something. ENACT picks the
> machine on the day you deploy, and the grid changes every half hour. When that machine's
> power turns dirty, ENACT notices and names a better one. The app does not move. It keeps
> running where it was, below its own sixty percent.

## 0:40 to 3:00, the comparison (switch to the screen, press "Play the day")

> This is live, on the challenge cluster, on this laptop. The same app is running twice. Both
> copies get the same traffic, five real requests a second.
>
> The three bars at the top are the day. They use real figures from the British grid for the
> twenty-ninth of September, sped up so half an hour passes every four seconds. Machine A runs
> on the North East's power, Machine B on the North West's. Yellow means clean, grey means
> dirty. The third bar is the machine ENACT picks.
>
> On the left is the standard build, which is what the checklist gives you. On the right, the
> same app with Sunflower.

*(about 0:11 on your stopwatch after pressing Play: ENACT's pick changes to Machine B)*

> ENACT has just changed its pick to Machine B. Machine A is still above sixty percent though,
> and B is only a little better, so Sunflower stays put. It does not chase small differences.

*(about 0:21: Machine A's bar turns grey, and both timers start counting)*

> Now Machine A has dropped below sixty percent. Both copies are on dirty power and both
> timers are running. Look at the line at the bottom of each panel: the app's own check,
> built with ENACT's Application Controller, says "relocate" on both sides. Sunflower waits
> eight seconds to be sure it is not a blip.

*(about 0:33: the right-hand bar shows Machine B, and the sunflower marker appears)*

> And it has moved. It started a second copy on Machine B, waited until that copy was
> answering, then let the first one go. Look at the bottom right: every request answered,
> zero lost.
>
> The timer on the right stopped at twelve seconds. Eight of those were Sunflower being
> careful, and three and a half were the move. The app was answering the whole time.
>
> The left-hand timer is still running, because that copy is still on Machine A.

*(let it run to about 1:15, then talk over the rest)*

> By the end of the day's data, the standard build has spent forty-eight seconds on dirty
> power and Sunflower twelve. That is the replay, which runs the day four hundred and fifty
> times faster. At real speed the left side is six hours. The right side is about twenty-four
> seconds, because Sunflower's wait and its move take the same time however slowly the grid
> changes.

## 3:00 to 3:50, why it is safe (stay on the screen)

> Sunflower does not choose where the app goes. ENACT chooses, and Sunflower reads that choice
> and carries it out, using the same pin ENACT's plug-in uses. You switch it on for an app
> with one line.
>
> It is careful about when. It ignores short dips. It ignores near-ties, because we measured
> ENACT's pick flipping twice in two and a half minutes between two equal machines. It limits
> how often it moves, and if a move fails it puts the app back.
>
> We tested that with thirty moves in a row and nearly eighteen hundred requests. None were
> lost.

## 3:50 to 4:40, what we give back (Highlights slide)

> To get here we had to read ENACT's source and run into its rough edges. We wrote every one
> down with the fix we used: thirty-nine findings. Two examples. On a fresh cluster the policy
> never picks a machine at all, for four separate reasons, and our setup script fixes all
> four. And ENACT's own assistant gets stuck in a loop on default settings, because half of
> its instructions are cut off before the model reads them. We found the cause and the fix.
>
> One thing is still open, and I would rather say so: the dataspace step needs connection
> details we have not been given yet.

## 4:40 to 5:00, close

> The repo has the raw data behind every number I quoted, and a recording of this run you can
> play in a browser. Thank you. I'm happy to take questions.

## If the live screen misbehaves

Say "I'll show you the recorded run, it is the same data" and open the recording:
`python3 -m http.server 8000 --directory docs/demo`, then <http://localhost:8000>. It plays
the same picture from the saved measurements. Do not debug on the call.

## Why it is called Sunflower

If someone asks: a sunflower turns to face the sun through the day. This turns an app toward
whichever machine has the cleanest power.

## Words you might be asked about

| Word | Plain meaning |
|---|---|
| Cluster | a group of computers that run apps together; ours is three, simulated on one laptop |
| Node | one of those computers; on screen they are Machine A and Machine B |
| Kubernetes | the system that starts apps on those computers and keeps them running |
| Policy, RuntimePolicy | the rules you give ENACT: which region, how green |
| Policy operator | the part of ENACT that reads the policy and names the best machine |
| Pin | telling Kubernetes "run this app on that exact machine" |
| Controller | a small program that watches for a change and reacts; Sunflower is one |
| Hard and Soft rules | Hard must never be broken; Soft is a preference |
| SDK, plug-in | ENACT's add-on for the Eclipse editor, with the wizards we clicked through |
