# The pitch

Five minutes, strict. About 600 words spoken, so the screen has time to do its work. The
times are where you should be when each part ends.

Before you start: restart the laptop, double-click `Set the stage.command` in the project
folder (it runs `./deploy/scene.sh`), open <http://localhost:35590>
full screen, and keep the slides in a second tab. Leave Eclipse, Ollama and spare browser tabs
closed. Play the day once as a warm-up, then set the stage again for the real one. The stage has to
be set before every run: after a run the two copies are on different machines.

If the live run comes out slower than rehearsed, read out what the screen says under the
right-hand bar. It splits the time into ENACT's part, Sunflower's wait and the move, so you
can say whose delay it was.

## The short version

About 190 words, in short sentences, so the screen does most of the talking. Say each line,
then stop and let people look. Silence while the screen plays is fine. If you use this
version, the longer one below is background for questions.

**Title slide**

> Hi, I'm ___. This is Sunflower.
>
> ENACT picks the best machine for an app. It picks once, on deploy day.
> When that machine's power turns dirty, the app stays there.
> Sunflower moves it.

**The screen. Press "Play the day".**

> This is live. It is the same app, twice. Left is the standard build. Right has Sunflower.
>
> The top bars are real grid data from Britain. Yellow is clean power. Grey is dirty.

*(ENACT's pick changes to Machine B)*

> ENACT changed its pick. Sunflower stays. The gain is too small.

*(Machine A turns grey)*

> Now Machine A is dirty. Both numbers start counting. Sunflower waits eight seconds, to be sure.

*(the right-hand bar shows Machine B)*

> It moved. Zero requests lost.

*(let the day finish)*

> Left: six hours on dirty power. Right: twenty-four seconds.
> Those are real speed. The grey numbers beside them, forty-eight seconds and twelve, are
> this fast replay.
>
> Sunflower never picks the machine. ENACT does. Sunflower follows it, carefully.

**Highlights slide**

> The full checklist is done in the ENACT plug-in, with the dataspace.
> We tested thirty moves in a row. Zero requests lost.
> We also wrote down forty-five problems we found in ENACT, each with a fix.
>
> Everything is in the repo. Thank you.

## The longer version

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

*(about 0:21: Machine A's bar turns grey, and both big numbers start counting)*

> Now Machine A has dropped below sixty percent. Both copies are on dirty power and both
> numbers are counting. Look at the line at the bottom of each panel: the app's own check,
> built with ENACT's Application Controller, says "relocate" on both sides. Sunflower waits
> eight seconds to be sure it is not a blip.

*(about 0:33: the right-hand bar shows Machine B, and the sunflower marker appears)*

> And it has moved. It started a second copy on Machine B, waited until that copy was
> answering, then let the first one go. Look at the bottom right: every request answered,
> zero lost.
>
> In the replay that took twelve seconds. Eight were Sunflower being careful, and three and
> a half were the move. The app was answering the whole time.
>
> The left-hand number is still climbing, because that copy is still on Machine A.

*(let it run to about 1:15, then talk over the rest)*

> Look at the two big numbers. At real speed the standard build spends six hours on dirty
> power. Sunflower spends about twenty-four seconds.
>
> The grey number beside each one is this replay, which runs the day four hundred and fifty
> times faster: forty-eight seconds against twelve. That looks like four to one, and it is not the
> comparison. The left side's time stretches with the day. Sunflower's wait and its move take
> the same time however slowly the grid changes.

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
> down with the fix we used: forty-five findings. Two examples. On a fresh cluster the policy
> never picks a machine at all, for four separate reasons, and our setup script fixes all
> four. And ENACT's own assistant gets stuck in a loop on default settings, because half of
> its instructions are cut off before the model reads them. We found the cause and the fix.
>
> And all five items of the challenge's own checklist are done through the ENACT plug-in,
> including the dataspace: the carbon figures GreenCharge is using right now came through
> ENACT's Data and Object Space this morning.

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
